from __future__ import annotations

import json
from pathlib import Path

import pytest

from surfdude import conformance
from surfdude.dialects import AgentTraderDialect, SimpleDialect

FIXTURES = Path(__file__).resolve().parents[2] / "fixtures"


def _case_ids(directory: Path) -> list[str]:
    spec = json.loads((directory / "cases.json").read_text())
    return [c["name"] for c in spec["cases"]]


def _cases(directory: Path) -> list[dict]:
    return json.loads((directory / "cases.json").read_text())["cases"]


@pytest.mark.parametrize(
    "case",
    _cases(FIXTURES / "simple"),
    ids=_case_ids(FIXTURES / "simple"),
)
def test_simple_dialect_matches_fixtures(case: dict) -> None:
    dialect = SimpleDialect()
    conformance._run_case(dialect, FIXTURES / "simple", case)


@pytest.mark.parametrize(
    "case",
    _cases(FIXTURES / "agenttrader"),
    ids=_case_ids(FIXTURES / "agenttrader"),
)
def test_agenttrader_dialect_matches_fixtures(case: dict) -> None:
    dialect = AgentTraderDialect()
    conformance._run_case(dialect, FIXTURES / "agenttrader", case)
