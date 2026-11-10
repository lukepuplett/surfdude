"""Challenge handlers: listing-weeks 402 triggers sandbox credit and retries publish."""

from __future__ import annotations

import threading

import httpx

from surfdude import AgentTraderDialect, Surfer
from surfdude.dialects.agenttrader import PROFILE_URI
from surfdude.dialects.agenttrader_match import match_capability_gap
from surfdude.surf.challenge import Challenge, ChallengeContext
from surfdude.surf.x402_match import match_x402_payment

PROFILE = f'<{PROFILE_URI}>; rel="profile"'


class ListingWeeksApi:
    def __init__(self) -> None:
        self.publish_hits = 0
        self.credited = False
        self._lock = threading.Lock()

        def handler(request: httpx.Request) -> httpx.Response:
            headers = {"Content-Type": "application/json", "Link": PROFILE}
            path = request.url.path
            if request.method == "GET" and path == "/":
                return httpx.Response(
                    200,
                    json={
                        "status": 200,
                        "message": "OK",
                        "data": [{"name": "test"}],
                        "_actions": [
                            {
                                "rel": "publish",
                                "action": "/listings/lst_1/publish",
                                "method": "POST",
                                "enctype": "application/json",
                                "fields": [
                                    {
                                        "name": "weeks",
                                        "required": True,
                                        "schema": {"type": "integer"},
                                    }
                                ],
                            }
                        ],
                    },
                    headers=headers,
                )
            if request.method == "POST" and path == "/sandbox/listing-advert-weeks":
                with self._lock:
                    self.credited = True
                return httpx.Response(
                    200,
                    json={"status": 200, "message": "OK", "data": [{"balance": 4}]},
                    headers=headers,
                )
            if request.method == "POST" and path == "/listings/lst_1/publish":
                with self._lock:
                    self.publish_hits += 1
                    first = self.publish_hits == 1
                if first and not self.credited:
                    return httpx.Response(
                        402,
                        json={
                            "status": 402,
                            "message": "Unable to publish the listing. Short of weeks.",
                            "missing": "listing-weeks",
                            "data": [
                                {
                                    "field": "weeks",
                                    "message": "need more",
                                }
                            ],
                            "_actions": [
                                {
                                    "rel": "sandbox-listing-advert-credit",
                                    "action": "/sandbox/listing-advert-weeks",
                                    "method": "POST",
                                    "fields": [{"name": "weeks", "required": True}],
                                }
                            ],
                        },
                        headers=headers,
                    )
                return httpx.Response(
                    200,
                    json={"status": 200, "message": "OK", "data": [{"state": "live"}]},
                    headers=headers,
                )
            return httpx.Response(404, json={"status": 404, "message": "Not found", "data": []}, headers=headers)

        self._transport = httpx.MockTransport(handler)

    def surfer(self) -> Surfer:
        def credit_weeks(ctx: ChallengeContext) -> tuple[bool, Exception | None]:
            if not ctx.page.has("sandbox-listing-advert-credit"):
                return False, None
            try:
                ctx.page.submit("sandbox-listing-advert-credit", {"weeks": 4})
            except Exception as exc:  # noqa: BLE001 — surface to surfer
                return False, exc
            return True, None

        credit = Challenge(
            name="listing-weeks",
            match=match_capability_gap(402, "listing-weeks"),
            handle=credit_weeks,
        )
        return Surfer.new(
            "http://test/",
            AgentTraderDialect(),
            client=httpx.Client(transport=self._transport),
            challenges=[credit],
        )


def test_listing_weeks_retries_publish() -> None:
    api = ListingWeeksApi()
    with api.surfer() as s:
        home = s.root()
        page = home.submit("publish", {"weeks": 2})
    assert page.status == 200
    assert api.publish_hits == 2
    assert api.credited


def test_match_capability_gap_ignores_x402_body() -> None:
    matcher = match_capability_gap(402, "listing-weeks")
    body = b'{"x402Version":1,"accepts":[{"scheme":"exact"}]}'
    assert not matcher(402, httpx.Headers(), body)


def test_match_x402_payment() -> None:
    body = b'{"x402Version":1,"accepts":[]}'
    assert match_x402_payment(402, None, body)
    assert not match_x402_payment(402, None, b'{"status":402,"data":[]}')
