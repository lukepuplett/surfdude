"""Opt-in retries.

A surfer retries nothing unless it is given a policy, because a tool that tests an API should see
every failure. A client that drives a real server can opt in with ``Surfer.new(retry=...)`` or per
step with ``retry=...`` on ``follow``, ``submit`` and the wave steps.
"""

from __future__ import annotations

import random
from collections.abc import Callable
from dataclasses import dataclass, field
from datetime import UTC, datetime
from email.utils import parsedate_to_datetime

import httpx


@dataclass(frozen=True)
class RetryAttempt:
    """An attempt that has just failed, given to a retry policy so it can decide what to do."""

    number: int
    """How many attempts have been made so far, counting from 1."""
    method: str
    status: int = 0
    """The status the server answered with, or 0 when the request itself failed."""
    headers: httpx.Headers | None = None
    error: Exception | None = None
    """The transport error, or None when the server answered."""
    idempotent: bool = False
    """True when repeating the request is safe: the method is idempotent, or it carries an
    Idempotency-Key."""


RetryPolicy = Callable[[RetryAttempt], float | None]
"""A function that returns the seconds to wait before trying again, or None to give up."""


def never(attempt: RetryAttempt) -> None:
    """The policy that retries nothing. A step passes it to turn off the surfer's policy."""
    return


@dataclass
class StandardRetry:
    """A conservative preset for clients of real servers.

    Three attempts, exponential backoff with jitter from 0.2s up to 5s, on 429, 502, 503, 504 and
    connection errors, for idempotent requests only. A ``Retry-After`` longer than ``max_delay`` is
    treated as a refusal rather than waited for.
    """

    max_attempts: int = 3
    base_delay: float = 0.2
    max_delay: float = 5.0
    statuses: tuple[int, ...] = (429, 502, 503, 504)
    jitter: Callable[[], float] = field(default=random.random)
    """Returns a number in [0, 1) that scales each backoff."""

    def __call__(self, attempt: RetryAttempt) -> float | None:
        if attempt.number >= self.max_attempts or not attempt.idempotent:
            return None
        if attempt.error is None and attempt.status not in self.statuses:
            return None

        wait = _retry_after(attempt.headers)
        if wait is not None:
            # The server said when to come back. Coming back sooner is rude, and waiting too long
            # stalls the caller, so a request for more than the cap is treated as a refusal.
            return wait if wait <= self.max_delay else None

        backoff = min(self.base_delay * 2 ** (attempt.number - 1), self.max_delay)
        return self.jitter() * backoff


def _retry_after(headers: httpx.Headers | None) -> float | None:
    """Read a Retry-After header, either a number of seconds or an HTTP date."""
    value = headers.get("Retry-After", "").strip() if headers is not None else ""
    if not value:
        return None
    if value.isdigit():
        return float(value)
    try:
        when = parsedate_to_datetime(value)
    except (TypeError, ValueError):
        return None
    if when.tzinfo is None:
        when = when.replace(tzinfo=UTC)
    return max(0.0, (when - datetime.now(UTC)).total_seconds())


_IDEMPOTENT_METHODS = frozenset({"GET", "HEAD", "OPTIONS", "PUT", "DELETE", "TRACE"})


def is_idempotent(request: httpx.Request) -> bool:
    """True when repeating the request is safe."""
    return request.method in _IDEMPOTENT_METHODS or "Idempotency-Key" in request.headers
