"""What the model has no field for is kept as an extension, as the server sent it."""

from __future__ import annotations

import json

from surfdude import AgentTraderDialect
from surfdude.dialect import HTTPResponse

BODY = {
    "status": 200,
    "message": "OK",
    "data": [],
    "_links": [{"rel": "human", "href": "/h", "method": "GET", "type": "text/html"}],
    "_actions": [
        {
            "rel": "buy",
            "action": "/buy",
            "method": "POST",
            "enctype": "application/json",
            "ifMatch": "v1",
            "title": "Buy it",
            "requires": ["agent-session"],
            "price": {"amount": "0.50", "currency": "USDC"},
            "fields": [],
        }
    ],
}


def _read() -> object:
    response = HTTPResponse(
        status_code=200,
        headers={"Content-Type": ["application/json"]},
        body=json.dumps(BODY).encode(),
    )
    return AgentTraderDialect().read(response)


def test_the_other_keys_of_a_control_are_extensions_with_the_servers_values() -> None:
    buy = _read().control("buy")

    assert buy.extensions["title"] == "Buy it"
    assert buy.extensions["requires"] == ["agent-session"]
    assert buy.extensions["price"] == {"amount": "0.50", "currency": "USDC"}


def test_what_changes_the_request_is_a_field_and_not_an_extension() -> None:
    buy = _read().control("buy")

    for core in ("rel", "action", "method", "enctype", "ifMatch", "fields"):
        assert core not in buy.extensions, f"{core} changes the request, so it is a field"
    assert (buy.method, buy.enctype, buy.if_match) == ("POST", "application/json", "v1")


def test_a_links_type_is_an_extension() -> None:
    assert _read().control("human").extensions["type"] == "text/html"
