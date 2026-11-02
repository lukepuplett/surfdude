from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any

from surfdude.surf.values import Values

if TYPE_CHECKING:
    from surfdude.surf.surfer import ItemPage, Page, Surfer


@dataclass
class ReportLine:
    step: str
    url: str = ""
    status: int = 0
    duration: float = 0.0


@dataclass
class Report:
    lines: list[ReportLine] = field(default_factory=list)


@dataclass
class Wave:
    surfer: Surfer
    _steps: list[tuple[str, Any]] = field(default_factory=list)

    def from_root(self) -> Wave:
        def run(state: _WaveState) -> None:
            state.page = self.surfer.root()
            state.item = None

        return self._add("from root", run)

    def then(self, rel: str, *, values: Values | None = None) -> Wave:
        return self._add(f"then {rel}", lambda st: st.move(rel, values))

    def then_submit(self, rel: str, values: Values) -> Wave:
        return self._add(f"then submit {rel}", lambda st: st.move(rel, dict(values)))

    def then_item(self, index: int) -> Wave:
        def run(state: _WaveState) -> None:
            if state.page is None:
                raise RuntimeError(f"Unable to select item {index}. The wave has no page yet.")
            state.item = state.page.item(index)

        return self._add(f"then item {index}", run)

    def then_read(self, into: Any) -> Wave:
        def run(state: _WaveState) -> None:
            if state.item is not None:
                state.item.read(into)
            elif state.page is not None:
                state.page.read(into)
            else:
                raise RuntimeError("Unable to read. The wave has no page yet.")

        return self._add("then read", run)

    def _add(self, name: str, run) -> Wave:
        self._steps.append((name, run))
        return self

    def go(self) -> Report:
        report = Report()
        state = _WaveState(surfer=self.surfer)
        for name, run in self._steps:
            start = time.monotonic()
            try:
                run(state)
            except Exception as exc:
                line = _line(name, state, time.monotonic() - start)
                report.lines.append(line)
                raise RuntimeError(f'The wave stopped at "{name}": {exc}') from exc
            report.lines.append(_line(name, state, time.monotonic() - start))
        return report


@dataclass
class _WaveState:
    surfer: Surfer
    page: Page | None = None
    item: ItemPage | None = None

    def move(self, rel: str, values: Values | None) -> None:
        if self.item is not None:
            if values is not None:
                self.page = self.item.submit(rel, values)
            else:
                self.page = self.item.follow(rel, values=values)
        elif self.page is not None:
            if values is not None:
                self.page = self.page.submit(rel, values)
            else:
                self.page = self.page.follow(rel, values=values)
        else:
            raise RuntimeError("The wave has no page yet.")
        self.item = None


def _line(name: str, state: _WaveState, duration: float) -> ReportLine:
    line = ReportLine(step=name, duration=duration)
    if state.page is not None:
        line.url = state.page.url
        line.status = state.page.status
    return line
