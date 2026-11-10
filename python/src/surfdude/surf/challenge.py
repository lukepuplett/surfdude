from __future__ import annotations

import contextvars
import threading
from collections.abc import Callable
from dataclasses import dataclass
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from surfdude.surf.surfer import Page, Surfer

_challenge_lock = threading.Lock()

_challenging: contextvars.ContextVar[bool] = contextvars.ContextVar("surfdude_challenging", default=False)

ChallengeMatcher = Callable[[int, Any, bytes], bool]
ChallengeHandler = Callable[["ChallengeContext"], tuple[bool, Exception | None]]


@dataclass(frozen=True)
class Challenge:
    name: str
    match: ChallengeMatcher | None
    handle: ChallengeHandler | None


@dataclass
class ChallengeContext:
    surfer: Surfer
    step: str
    page: Page


def challenging() -> bool:
    return _challenging.get()


def try_challenges(
    surfer: Surfer,
    step: str,
    refused: Page,
    challenges: list[Challenge],
) -> tuple[bool, Exception | None]:
    """Run the first matching challenge handler on a refused page."""
    if not challenges or _challenging.get():
        return False, None

    body = refused.raw_body()
    headers = refused.headers
    status = refused.status

    for ch in challenges:
        if ch.match is None or ch.handle is None:
            continue
        if not ch.match(status, headers, body):
            continue
        with _challenge_lock:
            marker = _challenging.set(True)
            try:
                retry, err = ch.handle(
                    ChallengeContext(surfer=surfer, step=step, page=refused)
                )
            finally:
                _challenging.reset(marker)
            if err is not None:
                return False, err
            return retry, None
    return False, None
