from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any

Schema = dict[str, Any]


@dataclass
class Input:
    name: str
    required: bool = False
    value: Any = None
    schema: Schema = field(default_factory=dict)


@dataclass
class Control:
    """Something to follow or submit, found by ``rel``.

    Only what changes the request, or how the control is found, has a field: ``rel``, ``href``,
    ``method``, ``enctype``, ``inputs`` and ``if_match``. Anything else a dialect finds on a control,
    such as what it requires or what it costs, is an extension: kept under the dialect's own name,
    with the server's value as it was sent. The surfer never reads extensions.
    """

    rel: str
    href: str = ""
    method: str = ""
    enctype: str = ""
    form: bool = False
    inputs: list[Input] = field(default_factory=list)
    if_match: str = ""
    extensions: dict[str, Any] = field(default_factory=dict)

    def http_method(self) -> str:
        return (self.method or "GET").upper()


@dataclass
class Paging:
    skip: int = 0
    take: int = 0
    count: int = 0
    total: int | None = None


@dataclass
class Problem:
    field: str = ""
    code: str = ""
    message: str = ""


@dataclass
class Item:
    controls: list[Control] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)
    data: bytes = b""

    def control(self, rel: str) -> Control | None:
        return _find(self.controls, rel)

    def rels(self) -> list[str]:
        return [c.rel for c in self.controls]

    def decode(self, into: Any) -> None:
        parsed = json.loads(self.data)
        if isinstance(into, dict) and isinstance(parsed, dict):
            into.clear()
            into.update(parsed)
            return
        if isinstance(parsed, dict):
            for k, v in parsed.items():
                if hasattr(into, k):
                    setattr(into, k, v)


@dataclass
class Resource:
    status: int = 0
    message: str = ""
    missing: str = ""
    notes: list[str] = field(default_factory=list)
    paging: Paging | None = None
    controls: list[Control] = field(default_factory=list)
    items: list[Item] = field(default_factory=list)
    problems: list[Problem] = field(default_factory=list)
    body: bytes = b""

    def control(self, rel: str) -> Control | None:
        return _find(self.controls, rel)

    def rels(self) -> list[str]:
        return [c.rel for c in self.controls]

    def duplicate_rels(self) -> list[str]:
        counts: dict[str, int] = {}
        for c in self.controls:
            if c.rel:
                counts[c.rel] = counts.get(c.rel, 0) + 1
        return sorted(rel for rel, n in counts.items() if n > 1)

    def decode(self, into: Any) -> None:
        if self.problems:
            from surfdude.hypertext.errors import NoDataError

            raise NoDataError()
        if not self.items:
            from surfdude.hypertext.errors import NoDataError

            raise NoDataError()
        self.items[0].decode(into)


def _find(controls: list[Control], rel: str) -> Control | None:
    for c in controls:
        if c.rel == rel:
            return c
    return None
