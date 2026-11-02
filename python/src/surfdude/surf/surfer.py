from __future__ import annotations

import time
import uuid
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any, Self
from urllib.parse import urlparse

import httpx

from surfdude.dialect import Dialect, Registry
from surfdude.hypertext import (
    Control,
    InvalidInputError,
    MissingInputError,
    MultiItemReadError,
    RelNotFoundError,
    RequestFailedError,
    Resource,
    UnexpectedInputError,
    UnexpectedStatusError,
)
from surfdude.surf.preflight import preflight
from surfdude.surf.request import build_request, response_from_httpx
from surfdude.surf.retry import RetryAttempt, RetryPolicy, is_idempotent, never
from surfdude.surf.values import Values, merge

if TYPE_CHECKING:
    from surfdude.surf.wave import Wave

MAX_BODY = 4 << 20


@dataclass
class Page:
    surfer: Surfer
    url: str
    status: int
    headers: httpx.Headers
    dialect: str = ""
    resource: Resource | None = None
    attempts: int = 1
    """How many times the request was sent. More than 1 only when a retry policy tried again."""

    def rels(self) -> list[str]:
        assert self.resource
        return self.resource.rels()

    def has(self, rel: str) -> bool:
        assert self.resource
        return self.resource.control(rel) is not None

    def follow(
        self,
        rel: str,
        *,
        values: Values | None = None,
        expect: Callable[[int], bool] | None = None,
        retry: RetryPolicy | None = None,
        idempotency_key: bool = False,
    ) -> Page:
        return self._take(
            self.resource.controls if self.resource else [],
            "This page",
            rel,
            values,
            expect,
            retry,
            idempotency_key,
        )

    def submit(
        self,
        rel: str,
        values: Values,
        *,
        expect: Callable[[int], bool] | None = None,
        retry: RetryPolicy | None = None,
        idempotency_key: bool = False,
    ) -> Page:
        return self._take(
            self.resource.controls if self.resource else [],
            "This page",
            rel,
            dict(values),
            expect,
            retry,
            idempotency_key,
        )

    def item(self, index: int) -> ItemPage:
        assert self.resource
        if index < 0 or index >= len(self.resource.items):
            raise ValueError(
                f"Unable to select item {index}. The page has {len(self.resource.items)}."
            )
        return ItemPage(page=self, index=index, item=self.resource.items[index])

    def read(self, into: Any) -> None:
        assert self.resource
        if len(self.resource.items) > 1:
            raise MultiItemReadError(len(self.resource.items))
        self.resource.decode(into)

    def raw_body(self) -> bytes:
        if self.resource is None:
            return b""
        return self.resource.body

    def _take(
        self,
        controls: list[Control],
        where: str,
        rel: str,
        values: Values | None,
        expect: Callable[[int], bool] | None,
        retry: RetryPolicy | None = None,
        idempotency_key: bool = False,
    ) -> Page:
        control = _find_control(controls, rel)
        if control is None:
            offered = [c.rel for c in controls]
            raise RelNotFoundError(rel=rel, where=where, offered=offered)
        preflight(control, self.surfer.token)
        vals, err = _safe_merge(control, values)
        if err:
            raise err
        req = build_request(self.surfer.registry, self.url, control, vals, self.surfer.token)
        if idempotency_key:
            req.headers["Idempotency-Key"] = uuid.uuid4().hex
        return self.surfer._do(req, f'follow "{rel}"', expect, retry)


@dataclass
class ItemPage:
    page: Page
    index: int
    item: Any

    def rels(self) -> list[str]:
        return self.item.rels()

    def has(self, rel: str) -> bool:
        return self.item.control(rel) is not None

    def follow(
        self,
        rel: str,
        *,
        values: Values | None = None,
        expect: Callable[[int], bool] | None = None,
        retry: RetryPolicy | None = None,
        idempotency_key: bool = False,
    ) -> Page:
        return self.page._take(
            self.item.controls,
            f"Item {self.index}",
            rel,
            values,
            expect,
            retry,
            idempotency_key,
        )

    def submit(
        self,
        rel: str,
        values: Values,
        *,
        expect: Callable[[int], bool] | None = None,
        retry: RetryPolicy | None = None,
        idempotency_key: bool = False,
    ) -> Page:
        return self.page._take(
            self.item.controls,
            f"Item {self.index}",
            rel,
            dict(values),
            expect,
            retry,
            idempotency_key,
        )

    def read(self, into: Any) -> None:
        self.item.decode(into)


def _find_control(controls: list[Control], rel: str) -> Control | None:
    for c in controls:
        if c.rel == rel:
            return c
    return None


def _safe_merge(control: Control, values: Values | None) -> tuple[dict[str, Any], Exception | None]:
    try:
        return merge(control, values), None
    except (MissingInputError, InvalidInputError, UnexpectedInputError) as exc:
        return {}, exc


@dataclass
class Surfer:
    root_url: str
    registry: Registry
    client: httpx.Client = field(default_factory=httpx.Client)
    token: str = ""
    agent: str = "surfdude-python"
    retry: RetryPolicy | None = None
    sleep: Callable[[float], None] = field(default=time.sleep)

    @classmethod
    def new(
        cls,
        root: str,
        *dialects: Dialect,
        client: httpx.Client | None = None,
        retry: RetryPolicy | None = None,
        timeout: float = 30.0,
        sleep: Callable[[float], None] | None = None,
    ) -> Surfer:
        if not dialects:
            raise ValueError("Unable to start. A Surfer needs at least one dialect.")
        parsed = urlparse(root)
        if not parsed.scheme or not parsed.netloc:
            raise ValueError(f"Unable to start. {root!r} is not an absolute URL.")
        reg = Registry(dialects=list(dialects))
        return cls(
            root_url=root,
            registry=reg,
            client=client or httpx.Client(timeout=timeout),
            retry=retry,
            sleep=sleep or time.sleep,
        )

    def set_bearer(self, token: str) -> None:
        self.token = token

    def root(self) -> Page:
        req = httpx.Request("GET", self.root_url, headers=self._base_headers())
        return self._do(req, "open the front door", None)

    def open(self, url: str) -> Page:
        resolved = httpx.URL(self.root_url).join(url)
        req = httpx.Request("GET", str(resolved), headers=self._base_headers())
        return self._do(req, f"open {url!r}", None)

    def _base_headers(self) -> dict[str, str]:
        h = {"User-Agent": self.agent}
        accept = self.registry.accept()
        if accept:
            h["Accept"] = accept
        if self.token:
            h["Authorization"] = f"Bearer {self.token}"
        return h

    def _send(self, req: httpx.Request) -> tuple[httpx.Response | None, Exception | None]:
        """One attempt: the response with its body read, or the transport error."""
        try:
            resp = self.client.send(req)
            resp.read()
        except httpx.TransportError as exc:
            return None, exc
        return resp, None

    def _do(
        self,
        req: httpx.Request,
        step: str,
        expect: Callable[[int], bool] | None,
        retry: RetryPolicy | None = None,
    ) -> Page:
        """Send a request and read the response, trying again when the retry policy says to.

        A status the step expects is never retried, so a journey that tests a refusal sees it once.
        ``retry`` is None to use the surfer's policy, which is none unless one was given.
        """
        if expect is None:
            expect = lambda code: 200 <= code < 300
        policy = retry if retry is not None else self.retry
        if policy is None:
            policy = never

        attempts = 1
        while True:
            resp, error = self._send(req)
            if error is None and expect(resp.status_code):
                break
            wait = policy(
                RetryAttempt(
                    number=attempts,
                    method=req.method,
                    status=resp.status_code if resp is not None else 0,
                    headers=resp.headers if resp is not None else None,
                    error=error,
                    idempotent=is_idempotent(req),
                )
            )
            if wait is None:
                break
            self.sleep(wait)
            attempts += 1

        if error is not None:
            raise RequestFailedError(step=step, cause=error, attempts=attempts) from error
        assert resp is not None

        body = resp.content
        if len(body) > MAX_BODY:
            body = body[:MAX_BODY]
        page = Page(
            surfer=self,
            url=str(resp.url),
            status=resp.status_code,
            headers=resp.headers,
            attempts=attempts,
        )
        msg = response_from_httpx(resp)
        msg.body = body
        dialect = self.registry.choose(msg)
        if dialect is None:
            if not expect(resp.status_code):
                raise UnexpectedStatusError(
                    step=step, status=resp.status_code, attempts=attempts, url=page.url
                )
            raise self.registry.unsupported(msg)
        page.dialect = dialect.name()
        resource = dialect.read(msg)
        page.resource = resource
        if not expect(resp.status_code):
            raise UnexpectedStatusError(
                step=step,
                status=resp.status_code,
                message=resource.message,
                resource=resource,
                attempts=attempts,
                url=page.url,
            )
        return page

    def close(self) -> None:
        self.client.close()

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *args: object) -> None:
        self.close()

    def wave(self) -> Wave:
        from surfdude.surf.wave import Wave

        return Wave(self)
