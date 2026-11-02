"""Tests for the opt-in retry policy.

Nothing is retried unless a policy is given, and a retry is step-aware: a status the step expects is
never retried.
"""

from __future__ import annotations

from collections import defaultdict
from collections.abc import Callable
from datetime import UTC, datetime, timedelta
from email.utils import format_datetime
from typing import Any

import httpx
import pytest

from surfdude import AgentTraderDialect, Surfer
from surfdude.hypertext import RequestFailedError, UnexpectedStatusError
from surfdude.surf.retry import RetryAttempt, StandardRetry, never
from surfdude.surf.wave import WaveStoppedError

PROFILE = '<https://agenttrader.net/profiles/envelope/v1>; rel="profile"'

HOME: dict[str, Any] = {
    "status": 200,
    "message": "OK",
    "data": [],
    "_links": [
        {"rel": "flaky", "href": "/flaky", "method": "GET"},
        {"rel": "limited", "href": "/limited", "method": "GET"},
    ],
    "_actions": [
        {
            "rel": "shortlist",
            "action": "/shortlist",
            "method": "POST",
            "enctype": "application/json",
            "fields": [{"name": "note", "required": False, "schema": {"type": "string"}}],
        }
    ],
}

Script = Callable[[str, int, httpx.Request], httpx.Response | None]


def _json(status: int, body: dict[str, Any], headers: dict[str, str] | None = None) -> httpx.Response:
    all_headers = {"Content-Type": "application/json", "Link": PROFILE, **(headers or {})}
    return httpx.Response(status, json=body, headers=all_headers)


def _busy() -> httpx.Response:
    return _json(503, {"status": 503, "message": "busy", "data": []})


class Server:
    """A fake server that counts requests per path and lets a test script the answers."""

    def __init__(self, script: Script | None = None) -> None:
        self.script = script
        self.hits: defaultdict[str, int] = defaultdict(int)
        self.keys: defaultdict[str, list[str]] = defaultdict(list)

    def __call__(self, request: httpx.Request) -> httpx.Response:
        path = request.url.path
        self.hits[path] += 1
        self.keys[path].append(request.headers.get("Idempotency-Key", ""))
        if self.script is not None:
            scripted = self.script(path, self.hits[path], request)
            if scripted is not None:
                return scripted
        if path == "/":
            return _json(200, HOME)
        return _json(200, {"status": 200, "message": "OK", "data": []})


def fail_first(path: str, n: int, make: Callable[[], httpx.Response] = _busy) -> Script:
    def script(p: str, hit: int, _request: httpx.Request) -> httpx.Response | None:
        return make() if p == path and hit <= n else None

    return script


def make_surfer(server: Server, **options: Any) -> tuple[Surfer, list[float]]:
    waits: list[float] = []
    client = httpx.Client(transport=httpx.MockTransport(server), base_url="http://test")
    surfer = Surfer.new(
        "http://test/", AgentTraderDialect(), client=client, sleep=waits.append, **options
    )
    return surfer, waits


def steady(**changes: Any) -> StandardRetry:
    return StandardRetry(jitter=lambda: 1.0, **changes)


def test_default_is_no_retry() -> None:
    server = Server(fail_first("/flaky", 1))
    surfer, _ = make_surfer(server)

    with pytest.raises(UnexpectedStatusError) as exc:
        surfer.root().follow("flaky")

    assert exc.value.status == 503
    assert server.hits["/flaky"] == 1


def test_a_get_is_retried_until_it_succeeds_with_backoff() -> None:
    server = Server(fail_first("/flaky", 2))
    surfer, waits = make_surfer(server, retry=steady())

    page = surfer.root().follow("flaky")

    assert page.attempts == 3
    assert waits == [0.2, 0.4]


def test_it_gives_up_after_max_attempts() -> None:
    server = Server(fail_first("/flaky", 99))
    surfer, _ = make_surfer(server, retry=steady(max_attempts=2))

    with pytest.raises(UnexpectedStatusError) as exc:
        surfer.root().follow("flaky")

    assert exc.value.attempts == 2
    assert server.hits["/flaky"] == 2
    assert "tried 2 times" in str(exc.value)


def test_a_status_the_step_expects_is_never_retried() -> None:
    server = Server(fail_first("/flaky", 99))
    surfer, _ = make_surfer(server, retry=steady())

    page = surfer.root().follow("flaky", expect=lambda code: code == 503)

    assert page.attempts == 1
    assert server.hits["/flaky"] == 1


def test_a_step_can_turn_the_surfer_policy_off() -> None:
    server = Server(fail_first("/flaky", 1))
    surfer, _ = make_surfer(server, retry=steady())

    with pytest.raises(UnexpectedStatusError):
        surfer.root().follow("flaky", retry=never)

    assert server.hits["/flaky"] == 1


def test_a_step_can_turn_retries_on_for_a_surfer_with_none() -> None:
    server = Server(fail_first("/flaky", 1))
    surfer, _ = make_surfer(server)

    page = surfer.root().follow("flaky", retry=steady())

    assert page.attempts == 2


def test_a_post_is_not_repeated_by_default() -> None:
    server = Server(fail_first("/shortlist", 1))
    surfer, _ = make_surfer(server, retry=steady())

    with pytest.raises(UnexpectedStatusError):
        surfer.root().submit("shortlist", {"note": "x"})

    assert server.hits["/shortlist"] == 1
    assert server.keys["/shortlist"] == [""]


def test_a_post_that_carries_an_idempotency_key_is_repeated() -> None:
    server = Server(fail_first("/shortlist", 1))
    surfer, _ = make_surfer(server, retry=steady())

    page = surfer.root().submit("shortlist", {"note": "x"}, idempotency_key=True)

    keys = server.keys["/shortlist"]
    assert page.attempts == 2
    assert len(keys) == 2
    assert keys[0] != "" and keys[0] == keys[1]


def test_retry_after_seconds_are_honoured() -> None:
    server = Server(
        fail_first(
            "/limited",
            1,
            lambda: _json(429, {"status": 429, "message": "slow", "data": []}, {"Retry-After": "2"}),
        )
    )
    surfer, waits = make_surfer(server, retry=steady())

    surfer.root().follow("limited")

    assert waits == [2.0]


def test_a_retry_after_beyond_the_cap_is_not_waited_for() -> None:
    server = Server(
        fail_first(
            "/limited",
            99,
            lambda: _json(429, {"status": 429, "message": "slow", "data": []}, {"Retry-After": "120"}),
        )
    )
    surfer, waits = make_surfer(server, retry=steady())

    with pytest.raises(UnexpectedStatusError):
        surfer.root().follow("limited")

    assert waits == []
    assert server.hits["/limited"] == 1


def test_a_retry_after_http_date_is_honoured() -> None:
    soon = datetime.now(UTC) + timedelta(seconds=3)
    attempt = RetryAttempt(
        number=1,
        method="GET",
        status=429,
        headers=httpx.Headers({"Retry-After": format_datetime(soon, usegmt=True)}),
        idempotent=True,
    )

    wait = steady()(attempt)

    assert wait is not None and 1.0 <= wait <= 3.0


def test_a_status_outside_the_list_is_not_retried() -> None:
    attempt = RetryAttempt(number=1, method="GET", status=404, idempotent=True)

    assert steady()(attempt) is None


def _drop(_path: str = "/flaky", n: int = 1) -> Script:
    def raise_connect(path: str, hit: int, _request: httpx.Request) -> httpx.Response | None:
        if path == "/flaky" and hit <= n:
            raise httpx.ConnectError("connection reset")
        return None

    return raise_connect


def test_a_connection_error_is_repeated() -> None:
    server = Server(_drop())
    surfer, _ = make_surfer(server, retry=steady())

    page = surfer.root().follow("flaky")

    assert page.attempts == 2


def test_a_connection_error_without_a_policy_is_a_request_failed_error() -> None:
    server = Server(_drop())
    surfer, _ = make_surfer(server)

    with pytest.raises(RequestFailedError) as exc:
        surfer.root().follow("flaky")

    assert exc.value.attempts == 1
    assert "connection reset" in str(exc.value)


def test_connection_errors_that_never_clear_report_every_attempt() -> None:
    server = Server(_drop(n=99))
    surfer, _ = make_surfer(server, retry=steady())

    with pytest.raises(RequestFailedError) as exc:
        surfer.root().follow("flaky")

    assert exc.value.attempts == 3
    assert "after 3 attempts" in str(exc.value)


def test_attempts_appear_in_the_wave_report() -> None:
    server = Server(fail_first("/flaky", 2))
    surfer, _ = make_surfer(server, retry=steady())

    report = surfer.wave().from_root().then("flaky").go()

    assert report.lines[1].attempts == 3


def test_the_report_shows_the_failing_step_and_its_attempts() -> None:
    server = Server(fail_first("/flaky", 99))
    surfer, _ = make_surfer(server, retry=steady(max_attempts=2))

    with pytest.raises(WaveStoppedError) as exc:
        surfer.wave().from_root().then("flaky").go()

    last = exc.value.report.lines[-1]
    assert (last.status, last.attempts) == (503, 2)


def test_the_timeout_is_configurable() -> None:
    surfer = Surfer.new("http://test/", AgentTraderDialect(), timeout=5.0)

    assert surfer.client.timeout == httpx.Timeout(5.0)
    surfer.close()
