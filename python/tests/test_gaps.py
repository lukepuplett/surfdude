"""Regression tests for behaviour gaps that were found by review and then fixed.

Each test is named after its entry in ../tmp/gap-tracking.md.
"""

from __future__ import annotations

import json
from typing import Any

import httpx
import pytest

from surfdude import AgentTraderDialect, Surfer
from surfdude.hypertext import InvalidInputError, NoDataError
from surfdude.hypertext.errors import MultiItemReadError
from surfdude.surf.preflight import (
    NonHypermediaControlError,
    PaymentRequiredError,
    RequiresNotMetError,
)

PROFILE = '<https://agenttrader.net/profiles/envelope/v1>; rel="profile"'


def _json(status: int, body: dict[str, Any]) -> httpx.Response:
    return httpx.Response(
        status,
        json=body,
        headers={"Content-Type": "application/json", "Link": PROFILE},
    )


def test_gap01_requires_preflight_before_http() -> None:
    secret_hits = 0

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal secret_hits
        if request.url.path == "/secret":
            secret_hits += 1
        return _json(
            200,
            {
                "status": 200,
                "message": "OK",
                "data": [{"name": "Fake"}],
                "_links": [
                    {"rel": "self", "href": "/", "method": "GET"},
                    {
                        "rel": "secret",
                        "href": "/secret",
                        "method": "GET",
                        "requires": ["agent-session"],
                    },
                ],
            },
        )

    client = httpx.Client(transport=httpx.MockTransport(handler), base_url="http://test")
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=client)
    home = surfer.root()
    with pytest.raises(RequiresNotMetError) as exc:
        home.follow("secret")
    assert "agent-session" in str(exc.value).lower()
    assert secret_hits == 0


def test_gap02_price_preflight_before_post() -> None:
    post_hits = 0

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal post_hits
        if request.method == "POST" and request.url.path == "/paid":
            post_hits += 1
        return _json(
            200,
            {
                "status": 200,
                "message": "OK",
                "data": [],
                "_actions": [
                    {
                        "rel": "buy",
                        "action": "/paid",
                        "method": "POST",
                        "enctype": "application/json",
                        "price": {"amount": "1", "currency": "USD", "note": "fee"},
                        "fields": [{"name": "sku", "required": True, "schema": {"type": "string"}}],
                    }
                ],
            },
        )

    client = httpx.Client(transport=httpx.MockTransport(handler), base_url="http://test")
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=client)
    with pytest.raises(PaymentRequiredError) as exc:
        surfer.root().submit("buy", {"sku": "x"})
    assert "pay" in str(exc.value).lower()
    assert post_hits == 0


def test_gap03_non_hypermedia_type_preflight() -> None:
    html_hits = 0

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal html_hits
        if request.url.path == "/human":
            html_hits += 1
            return httpx.Response(200, text="<html></html>", headers={"Content-Type": "text/html"})
        return _json(
            200,
            {
                "status": 200,
                "message": "OK",
                "data": [],
                "_links": [
                    {"rel": "self", "href": "/", "method": "GET"},
                    {"rel": "human-page", "href": "/human", "method": "GET", "type": "text/html"},
                ],
            },
        )

    client = httpx.Client(transport=httpx.MockTransport(handler), base_url="http://test")
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=client)
    with pytest.raises(NonHypermediaControlError):
        surfer.root().follow("human-page")
    assert html_hits == 0


def test_gap04_read_on_list_page_must_select_item(gap_listings_server: httpx.Client) -> None:
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=gap_listings_server)
    home = surfer.root()
    results = home.submit("search-listings", {"make": "MINI"})
    assert len(results.resource.items) >= 2
    target: dict[str, Any] = {}
    with pytest.raises(MultiItemReadError) as exc:
        results.read(target)
    assert "item" in str(exc.value).lower()


def test_gap05_read_after_422_should_not_decode_as_business_dto(gap_listings_server: httpx.Client) -> None:
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=gap_listings_server)
    page = surfer.root().submit("create-listing", {"reg": ""}, expect=lambda s: s == 422)
    assert page.resource.problems
    business: dict[str, Any] = {}
    with pytest.raises(NoDataError):
        page.read(business)


def test_gap06_wave_from_arbitrary_url(gap_listings_server: httpx.Client) -> None:
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=gap_listings_server)
    report = surfer.wave().from_url("/listings").then_item(0).go()
    assert report.lines[0].url.endswith("/listings")
    assert report.lines[0].status == 200
    assert len(report.lines) == 2


def test_gap07_page_exposes_raw_response_body(gap_listings_server: httpx.Client) -> None:
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=gap_listings_server)
    home = surfer.root()
    envelope = json.loads(home.raw_body())
    assert envelope["status"] == 200
    assert envelope["_actions"][0]["rel"] == "search-listings"


def test_gap08_wave_expect_422(gap_listings_server: httpx.Client) -> None:
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=gap_listings_server)
    report = surfer.wave().from_root().then_submit("create-listing", {"reg": ""}, expect=422).go()
    assert [line.status for line in report.lines] == [200, 422]


def test_gap09_schema_format_validated() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return _json(
            200,
            {
                "status": 200,
                "message": "OK",
                "data": [],
                "_actions": [
                    {
                        "rel": "register",
                        "action": "/auth/start",
                        "method": "POST",
                        "enctype": "application/json",
                        "fields": [
                            {
                                "name": "email",
                                "required": True,
                                "schema": {"type": "string", "format": "email"},
                            }
                        ],
                    }
                ],
            },
        )

    client = httpx.Client(transport=httpx.MockTransport(handler), base_url="http://test")
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=client)
    with pytest.raises(InvalidInputError):
        surfer.root().submit("register", {"email": "not-an-email"})


def test_gap10_duplicate_rel_reported() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return _json(
            200,
            {
                "status": 200,
                "message": "OK",
                "data": [],
                "_links": [
                    {"rel": "self", "href": "/a", "method": "GET"},
                    {"rel": "self", "href": "/b", "method": "GET"},
                ],
            },
        )

    client = httpx.Client(transport=httpx.MockTransport(handler), base_url="http://test")
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=client)
    home = surfer.root()
    assert hasattr(home.resource, "duplicate_rels"), "Resource needs duplicate_rels()"
    assert home.resource.duplicate_rels()


@pytest.fixture
def gap_listings_server() -> httpx.Client:
    """Minimal AgentTrader fake: home, search list, create-listing 422."""

    def handler(request: httpx.Request) -> httpx.Response:
        path = request.url.path
        if request.method == "GET" and path in ("", "/"):
            return _json(
                200,
                {
                    "status": 200,
                    "message": "OK",
                    "data": [{"name": "Fake"}],
                    "_actions": [
                        {
                            "rel": "search-listings",
                            "action": "/listings",
                            "method": "GET",
                            "fields": [{"name": "make", "required": False, "schema": {"type": "string"}}],
                        },
                        {
                            "rel": "create-listing",
                            "action": "/listings",
                            "method": "POST",
                            "enctype": "application/json",
                            "fields": [{"name": "reg", "required": True, "schema": {"type": "string"}}],
                        },
                    ],
                },
            )
        if request.method == "GET" and path == "/listings":
            items = []
            for i in range(2):
                lid = f"lst_{i}"
                items.append(
                    {
                        "id": lid,
                        "_links": [{"rel": "self", "href": f"/listings/{lid}", "method": "GET"}],
                    }
                )
            return _json(200, {"status": 200, "message": "OK", "data": items})
        if request.method == "POST" and path == "/listings":
            body = json.loads(request.content.decode())
            if not body.get("reg"):
                return _json(
                    422,
                    {
                        "status": 422,
                        "message": "reg required",
                        "data": [{"field": "reg", "code": "required", "message": "is required"}],
                    },
                )
        return _json(404, {"status": 404, "message": "not found", "data": []})

    return httpx.Client(transport=httpx.MockTransport(handler), base_url="http://test")


def test_preflight_runs_before_input_validation() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return _json(
            200,
            {
                "status": 200,
                "message": "OK",
                "data": [],
                "_actions": [
                    {
                        "rel": "shortlist",
                        "action": "/shortlist",
                        "method": "POST",
                        "requires": ["agent-session"],
                        "fields": [{"name": "listingId", "required": True}],
                    }
                ],
            },
        )

    client = httpx.Client(transport=httpx.MockTransport(handler), base_url="http://test")
    surfer = Surfer.new("http://test/", AgentTraderDialect(), client=client)
    with pytest.raises(RequiresNotMetError):
        surfer.root().submit("shortlist", {})
