from __future__ import annotations

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
    Resource,
    UnexpectedInputError,
    UnexpectedStatusError,
)
from surfdude.surf.preflight import preflight
from surfdude.surf.request import build_request, response_from_httpx
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

    def rels(self) -> list[str]:
        assert self.resource
        return self.resource.rels()

    def has(self, rel: str) -> bool:
        assert self.resource
        return self.resource.control(rel) is not None

    def follow(self, rel: str, *, values: Values | None = None, expect: Callable[[int], bool] | None = None) -> Page:
        return self._take(self.resource.controls if self.resource else [], "This page", rel, values, expect)

    def submit(
        self,
        rel: str,
        values: Values,
        *,
        expect: Callable[[int], bool] | None = None,
    ) -> Page:
        merged = dict(values)
        return self._take(
            self.resource.controls if self.resource else [],
            "This page",
            rel,
            merged,
            expect,
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
        return self.surfer._do(req, f'follow "{rel}"', expect)


@dataclass
class ItemPage:
    page: Page
    index: int
    item: Any

    def rels(self) -> list[str]:
        return self.item.rels()

    def has(self, rel: str) -> bool:
        return self.item.control(rel) is not None

    def follow(self, rel: str, *, values: Values | None = None, expect: Callable[[int], bool] | None = None) -> Page:
        return self.page._take(
            self.item.controls,
            f"Item {self.index}",
            rel,
            values,
            expect,
        )

    def submit(
        self,
        rel: str,
        values: Values,
        *,
        expect: Callable[[int], bool] | None = None,
    ) -> Page:
        return self.page._take(
            self.item.controls,
            f"Item {self.index}",
            rel,
            dict(values),
            expect,
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

    @classmethod
    def new(cls, root: str, *dialects: Dialect, client: httpx.Client | None = None) -> Surfer:
        if not dialects:
            raise ValueError("Unable to start. A Surfer needs at least one dialect.")
        parsed = urlparse(root)
        if not parsed.scheme or not parsed.netloc:
            raise ValueError(f"Unable to start. {root!r} is not an absolute URL.")
        reg = Registry(dialects=list(dialects))
        return cls(
            root_url=root,
            registry=reg,
            client=client or httpx.Client(timeout=30.0),
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

    def _do(
        self,
        req: httpx.Request,
        step: str,
        expect: Callable[[int], bool] | None,
    ) -> Page:
        if expect is None:
            expect = lambda code: 200 <= code < 300
        resp = self.client.send(req)
        body = resp.read()
        if len(body) > MAX_BODY:
            body = body[:MAX_BODY]
        page = Page(
            surfer=self,
            url=str(resp.url),
            status=resp.status_code,
            headers=resp.headers,
        )
        msg = response_from_httpx(resp)
        msg.body = body
        dialect = self.registry.choose(msg)
        if dialect is None:
            if not expect(resp.status_code):
                raise UnexpectedStatusError(step=step, status=resp.status_code)
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
