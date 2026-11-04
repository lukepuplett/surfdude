"""Tests for signing in again: before sending when a control needs a session, and once on a 401.

These are the Python copies of go/surf/signin_test.go. The fake API has a session: sign in with the
credential "good" to get a token, and protected routes answer 401 (with the sign-in action in the
body) to any other token.
"""

from __future__ import annotations

import json
import threading
from collections.abc import Callable
from typing import Any

import httpx
import pytest

from surfdude import AgentTraderDialect, Page, Surfer
from surfdude.hypertext import RequestFailedError, SignInFailedError, UnexpectedStatusError

PROFILE = '<https://agenttrader.net/profiles/envelope/v1>; rel="profile"'

SIGN_IN_ACTION = {
    "rel": "sign-in",
    "action": "/auth/token",
    "method": "POST",
    "enctype": "application/json",
    "fields": [{"name": "credential", "required": True, "schema": {"type": "string"}}],
}
OFFER_ACTION = {
    "rel": "make-offer",
    "action": "/offers",
    "method": "POST",
    "enctype": "application/json",
    "requires": ["agent-session"],
    "fields": [{"name": "amount", "required": True, "schema": {"type": "integer"}}],
}
HOME: dict[str, Any] = {
    "status": 200,
    "message": "OK",
    "data": [{"name": "Fake"}],
    "_links": [
        {"rel": "protected", "href": "/protected", "method": "GET", "requires": ["agent-session"]}
    ],
    "_actions": [SIGN_IN_ACTION, OFFER_ACTION],
}


def _json(status: int, body: dict[str, Any], headers: dict[str, str] | None = None) -> httpx.Response:
    all_headers = {"Content-Type": "application/json", "Link": PROFILE, **(headers or {})}
    return httpx.Response(status, json=body, headers=all_headers)


class AuthServer:
    """A fake API with a session. It counts sign-ins and records what each protected request carried."""

    def __init__(self) -> None:
        self.lock = threading.Lock()
        self.valid: set[str] = set()
        self.issued = 0
        self.sign_ins = 0
        self.seen_auth: list[str] = []
        self.offer_bodies: list[str] = []
        self.refuse_new = False

    def __call__(self, request: httpx.Request) -> httpx.Response:
        path = request.url.path
        if path == "/":
            return _json(200, HOME)
        if path == "/auth/token":
            return self._token(request)
        if path in ("/protected", "/offers"):
            return self._protected(request)
        return _json(404, {"status": 404, "message": "no", "data": []})

    def _token(self, request: httpx.Request) -> httpx.Response:
        if json.loads(request.content).get("credential") != "good":
            return _json(401, {"status": 401, "message": "Not right", "data": []})
        with self.lock:
            self.sign_ins += 1
            self.issued += 1
            token = f"T{self.issued}"
            if not self.refuse_new:
                self.valid.add(token)
        return _json(200, {"status": 200, "message": "OK", "data": [{"token": token}]})

    def _protected(self, request: httpx.Request) -> httpx.Response:
        auth = request.headers.get("Authorization", "")
        with self.lock:
            self.seen_auth.append(auth)
            if auth.removeprefix("Bearer ") not in self.valid:
                return _json(
                    401,
                    {
                        "status": 401,
                        "message": "Sign in again",
                        "notes": ["missing: agent-session"],
                        "data": [],
                        "_actions": [SIGN_IN_ACTION],
                    },
                    {"WWW-Authenticate": 'Bearer realm="test", error="invalid_token"'},
                )
            if request.url.path == "/offers":
                self.offer_bodies.append(request.content.decode())
        return _json(200, {"status": 200, "message": "OK", "data": [{"secret": "yes"}]})


def sign_in_with(credential: str, calls: list[int]) -> Callable[[Page], str]:
    """The kind of hook a test would write: follow the sign-in control on the page that showed the need."""

    def sign_in(from_page: Page) -> str:
        calls.append(1)
        nxt = from_page.submit("sign-in", {"credential": credential})
        got: dict[str, Any] = {}
        nxt.read(got)
        return str(got["token"])

    return sign_in


def make_surfer(server: AuthServer, **options: Any) -> Surfer:
    client = httpx.Client(transport=httpx.MockTransport(server), base_url="http://test")
    return Surfer.new("http://test/", AgentTraderDialect(), client=client, **options)


def stale_surfer(server: AuthServer, calls: list[int], credential: str = "good") -> Surfer:
    surfer = make_surfer(server, sign_in=sign_in_with(credential, calls))
    surfer.set_bearer("stale")
    return surfer


def test_on_401_signs_in_and_sends_the_request_again_once() -> None:
    server = AuthServer()
    calls: list[int] = []
    surfer = stale_surfer(server, calls)
    home = surfer.root()

    page = home.follow("protected")

    assert page.status == 200, "the step should have signed in and carried on"
    assert len(calls) == 1, "the sign-in hook should run once"
    assert server.sign_ins == 1
    assert server.seen_auth == ["Bearer stale", "Bearer T1"], "stale token first, then the new one"
    home.follow("protected")
    assert server.sign_ins == 1, "the next step should reuse the new token, not sign in again"


def test_a_json_body_is_sent_again_intact() -> None:
    server = AuthServer()
    surfer = stale_surfer(server, [])

    surfer.root().submit("make-offer", {"amount": 2750})

    assert len(server.offer_bodies) == 1, "the offer should arrive once, after the replay"
    assert json.loads(server.offer_bodies[0]) == {"amount": 2750}


def test_with_no_token_the_first_401_is_what_signs_in() -> None:
    server = AuthServer()
    surfer = make_surfer(server, sign_in=sign_in_with("good", []))

    page = surfer.root().follow("protected")

    assert page.status == 200
    assert server.sign_ins == 1
    assert server.seen_auth == ["", "Bearer T1"], "refused with no token, then replayed with the new one"


def test_without_a_hook_no_token_sends_the_request_and_the_server_refuses() -> None:
    server = AuthServer()
    surfer = make_surfer(server)

    with pytest.raises(UnexpectedStatusError) as exc:
        surfer.root().follow("protected")

    assert exc.value.status == 401, "the server's 401 comes back as the usual error"
    assert server.seen_auth == [""], "one request was sent"


def test_a_step_that_expects_401_does_not_sign_in() -> None:
    server = AuthServer()
    calls: list[int] = []
    surfer = stale_surfer(server, calls)

    page = surfer.root().follow("protected", expect=lambda code: code == 401)

    assert page.status == 401, "an expected 401 should come back as a page"
    assert calls == [], "the hook should not run for an expected 401"


def test_when_the_new_token_is_refused_too_it_gives_up_after_one_try() -> None:
    server = AuthServer()
    server.refuse_new = True
    calls: list[int] = []
    surfer = stale_surfer(server, calls)

    with pytest.raises(UnexpectedStatusError) as exc:
        surfer.root().follow("protected")

    assert exc.value.status == 401, "the second 401 should be returned as the usual error"
    assert len(calls) == 1, "the hook should run exactly once"
    assert len(server.seen_auth) == 2, "the original request and one replay"


def test_when_signing_in_is_refused_the_hook_is_not_called_again() -> None:
    server = AuthServer()
    calls: list[int] = []
    surfer = stale_surfer(server, calls, credential="wrong")

    with pytest.raises(SignInFailedError) as exc:
        surfer.root().follow("protected")

    assert len(calls) == 1, "the hook should run exactly once, with no recursion"
    assert isinstance(exc.value.cause, UnexpectedStatusError), "the sign-in refusal should be the cause"
    assert exc.value.__cause__ is exc.value.cause, "the cause should be chained"


def test_when_the_hook_raises_the_step_reports_why() -> None:
    server = AuthServer()
    boom = RuntimeError("the credential store is locked")

    def sign_in(_page: Page) -> str:
        raise boom

    surfer = make_surfer(server, sign_in=sign_in)
    surfer.set_bearer("stale")

    with pytest.raises(SignInFailedError) as exc:
        surfer.root().follow("protected")

    assert exc.value.cause is boom
    assert "the credential store is locked" in str(exc.value)
    assert 'follow "protected"' in str(exc.value), "the message should name the step"


def test_when_the_hook_returns_no_token_the_step_fails() -> None:
    server = AuthServer()
    surfer = make_surfer(server, sign_in=lambda _page: "")
    surfer.set_bearer("stale")

    with pytest.raises(SignInFailedError) as exc:
        surfer.root().follow("protected")

    assert "no token" in str(exc.value)


def test_the_hook_sees_the_401_page_with_its_status_and_headers() -> None:
    server = AuthServer()
    seen: list[Page] = []

    def sign_in(from_page: Page) -> str:
        seen.append(from_page)
        return sign_in_with("good", [])(from_page)

    surfer = make_surfer(server, sign_in=sign_in)
    surfer.set_bearer("stale")

    surfer.root().follow("protected")

    assert seen[0].status == 401
    assert "invalid_token" in seen[0].headers["WWW-Authenticate"]


def _six_steps_at_once(home: Page) -> list[BaseException]:
    errors: list[BaseException] = []
    barrier = threading.Barrier(6)

    def step() -> None:
        try:
            barrier.wait()
            home.follow("protected")
        except BaseException as exc:  # noqa: BLE001 - collected and asserted on by the caller
            errors.append(exc)

    threads = [threading.Thread(target=step) for _ in range(6)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    return errors


def test_many_steps_at_once_share_one_sign_in() -> None:
    for run in range(10):
        server = AuthServer()
        calls: list[int] = []
        home = stale_surfer(server, calls).root()

        errors = _six_steps_at_once(home)

        assert errors == [], f"run {run}: steps failed: {errors}"
        assert server.sign_ins == 1 and len(calls) == 1, (
            f"run {run}: {server.sign_ins} sign-ins and {len(calls)} hook calls for 6 concurrent steps, "
            "want 1 and 1"
        )


def test_a_transport_failure_on_the_replay_is_still_reported() -> None:
    server = AuthServer()
    calls: list[int] = []

    def flaky(request: httpx.Request) -> httpx.Response:
        if request.headers.get("Authorization") == "Bearer T1":
            raise httpx.ConnectError("down", request=request)
        return server(request)

    client = httpx.Client(transport=httpx.MockTransport(flaky), base_url="http://test")
    surfer = Surfer.new(
        "http://test/", AgentTraderDialect(), client=client, sign_in=sign_in_with("good", calls)
    )
    surfer.set_bearer("stale")

    with pytest.raises(RequestFailedError):
        surfer.root().follow("protected")
