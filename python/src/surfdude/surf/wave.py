from __future__ import annotations

import time
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any

from surfdude.hypertext import RequestFailedError, UnexpectedStatusError
from surfdude.surf.retry import RetryPolicy
from surfdude.surf.values import Values

if TYPE_CHECKING:
    from surfdude.surf.surfer import ItemPage, Page, Surfer


def _expect_fn(expect: int | Callable[[int], bool] | None) -> Callable[[int], bool] | None:
    if expect is None:
        return None
    if isinstance(expect, int):
        return lambda code: code == expect
    return expect


@dataclass
class ReportLine:
    step: str
    url: str = ""
    status: int = 0
    attempts: int = 0
    duration: float = 0.0


@dataclass
class Report:
    lines: list[ReportLine] = field(default_factory=list)


class WaveStoppedError(RuntimeError):
    """A step failed. The report shows how far the wave got, including the failing step."""

    def __init__(self, message: str, report: Report) -> None:
        super().__init__(message)
        self.report = report


@dataclass
class Wave:
    surfer: Surfer
    _steps: list[tuple[str, Any]] = field(default_factory=list)

    def from_root(self) -> Wave:
        return self.from_url("")

    def from_url(self, url: str) -> Wave:
        label = "from root" if not url else f"from {url}"

        def run(state: _WaveState) -> None:
            if url:
                state.page = self.surfer.open(url)
            else:
                state.page = self.surfer.root()
            state.item = None

        return self._add(label, run)

    def then(
        self,
        rel: str,
        *,
        values: Values | None = None,
        expect: int | Callable[[int], bool] | None = None,
        retry: RetryPolicy | None = None,
        idempotency_key: bool = False,
    ) -> Wave:
        exp = _expect_fn(expect)
        return self._add(
            f"then {rel}", lambda st: st.move(rel, values, exp, retry, idempotency_key)
        )

    def then_submit(
        self,
        rel: str,
        values: Values,
        *,
        expect: int | Callable[[int], bool] | None = None,
        retry: RetryPolicy | None = None,
        idempotency_key: bool = False,
    ) -> Wave:
        exp = _expect_fn(expect)
        return self._add(
            f"then submit {rel}",
            lambda st: st.move(rel, dict(values), exp, retry, idempotency_key),
        )

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
                _describe_failure(line, exc)
                report.lines.append(line)
                raise WaveStoppedError(f'The wave stopped at "{name}": {exc}', report) from exc
            report.lines.append(_line(name, state, time.monotonic() - start))
        return report


@dataclass
class _WaveState:
    surfer: Surfer
    page: Page | None = None
    item: ItemPage | None = None

    def move(
        self,
        rel: str,
        values: Values | None,
        expect: Callable[[int], bool] | None,
        retry: RetryPolicy | None = None,
        idempotency_key: bool = False,
    ) -> None:
        options: dict[str, Any] = {
            "expect": expect,
            "retry": retry,
            "idempotency_key": idempotency_key,
        }
        if self.item is not None:
            if values is not None:
                self.page = self.item.submit(rel, values, **options)
            else:
                self.page = self.item.follow(rel, **options)
        elif self.page is not None:
            if values is not None:
                self.page = self.page.submit(rel, values, **options)
            else:
                self.page = self.page.follow(rel, **options)
        else:
            raise RuntimeError("The wave has no page yet.")
        self.item = None


def _line(name: str, state: _WaveState, duration: float) -> ReportLine:
    line = ReportLine(step=name, duration=duration)
    if state.page is not None:
        line.url = state.page.url
        line.status = state.page.status
        line.attempts = state.page.attempts
    return line


def _describe_failure(line: ReportLine, exc: Exception) -> None:
    """Show the failing response on the report, not the page the wave was on before it."""
    if isinstance(exc, UnexpectedStatusError):
        line.url = exc.url or line.url
        line.status = exc.status
        line.attempts = exc.attempts
    elif isinstance(exc, RequestFailedError):
        line.attempts = exc.attempts
