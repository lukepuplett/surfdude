from __future__ import annotations

from dataclasses import dataclass, field
from typing import Protocol

from surfdude.hypertext import Resource, UnsupportedMediaTypeError


@dataclass
class HTTPResponse:
    """Minimal response shape for dialect matching and reading."""

    status_code: int
    headers: dict[str, list[str]]
    body: bytes

    def header_get(self, name: str) -> str:
        for k, values in self.headers.items():
            if k.lower() == name.lower() and values:
                return values[0]
        return ""

    def header_values(self, name: str) -> list[str]:
        for k, values in self.headers.items():
            if k.lower() == name.lower():
                return values
        return []


class Dialect(Protocol):
    def name(self) -> str: ...

    def accept(self) -> str: ...

    def body_type(self) -> str: ...

    def matches(self, response: HTTPResponse) -> bool: ...

    def read(self, response: HTTPResponse) -> Resource: ...


@dataclass
class Registry:
    dialects: list[Dialect] = field(default_factory=list)

    def names(self) -> list[str]:
        return [d.name() for d in self.dialects]

    def accept(self) -> str:
        seen: set[str] = set()
        parts: list[str] = []
        for d in self.dialects:
            a = d.accept()
            if a and a not in seen:
                seen.add(a)
                parts.append(a)
        return ", ".join(parts)

    def choose(self, response: HTTPResponse) -> Dialect | None:
        for d in self.dialects:
            if d.matches(response):
                return d
        return None

    def first(self) -> Dialect | None:
        return self.dialects[0] if self.dialects else None

    def unsupported(self, response: HTTPResponse) -> UnsupportedMediaTypeError:
        return UnsupportedMediaTypeError(
            content_type=response.header_get("Content-Type"),
            tried=self.names(),
        )
