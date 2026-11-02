from __future__ import annotations

import json
from typing import Any

import httpx

from surfdude import AgentTraderDialect, Surfer

PROFILE = '<https://agenttrader.net/profiles/envelope/v1>; rel="profile"'


class FakeAgentTrader:
    def __init__(self) -> None:
        self.queries: list[str] = []
        self.bodies: dict[str, str] = {}

    def handler(self, request: httpx.Request) -> httpx.Response:
        path = request.url.path
        if request.method == "GET" and path in ("", "/"):
            body = {
                "status": 200,
                "message": "OK",
                "notes": ["Follow the links."],
                "data": [{"name": "Fake"}],
                "_links": [{"rel": "self", "href": "/", "method": "GET"}],
                "_actions": [
                    {
                        "rel": "search-listings",
                        "action": "/listings",
                        "method": "GET",
                        "fields": [
                            {"name": "make", "required": False, "schema": {"type": "string"}},
                            {"name": "maxPrice", "required": False, "schema": {"type": "integer"}},
                        ],
                    }
                ],
            }
            return self._json(200, body)
        if request.method == "GET" and path == "/listings":
            self.queries.append(request.url.query.decode())
            items = []
            for i in range(2):
                lid = f"lst_{i}"
                items.append(
                    {
                        "id": lid,
                        "_links": [{"rel": "self", "href": f"/listings/{lid}", "method": "GET"}],
                        "_actions": [
                            {
                                "rel": "shortlist",
                                "action": "/shortlist",
                                "method": "POST",
                                "enctype": "application/json",
                                "fields": [
                                    {"name": "listingId", "required": True, "value": lid},
                                    {"name": "note", "required": False, "schema": {"type": "string"}},
                                ],
                            }
                        ],
                    }
                )
            return self._json(
                200,
                {
                    "status": 200,
                    "message": "OK",
                    "data": items,
                    "paging": {"skip": 0, "take": 2, "count": 2, "total": 2},
                },
            )
        if request.method == "POST" and path == "/shortlist":
            self.bodies["shortlist"] = request.content.decode()
            return self._json(201, {"status": 201, "message": "Created", "data": [{"ok": True}]})
        return self._json(404, {"status": 404, "message": "not found", "data": []})

    def _json(self, status: int, body: dict[str, Any]) -> httpx.Response:
        return httpx.Response(
            status,
            json=body,
            headers={"Content-Type": "application/json", "Link": PROFILE},
        )


def test_wave_search_then_shortlist_knows_no_urls() -> None:
    fake = FakeAgentTrader()
    transport = httpx.MockTransport(fake.handler)
    client = httpx.Client(transport=transport, base_url="http://testserver")
    surfer = Surfer.new("http://testserver/", AgentTraderDialect(), client=client)

    report = (
        surfer.wave()
        .from_root()
        .then("search-listings", values={"make": "MINI", "maxPrice": 3500})
        .then_item(1)
        .then_submit("shortlist", {"note": "nice"})
        .go()
    )

    assert len(report.lines) == 4
    assert report.lines[-1].status == 201
    assert fake.queries == ["make=MINI&maxPrice=3500"]
    sent = json.loads(fake.bodies["shortlist"])
    assert sent["listingId"] == "lst_1"
    assert sent["note"] == "nice"
