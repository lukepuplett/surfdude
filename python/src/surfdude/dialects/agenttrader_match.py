from __future__ import annotations

import json
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

from surfdude.dialects.agenttrader import PROFILE_URI
from surfdude.surf.x402_match import is_x402_body


@dataclass(frozen=True)
class EnvelopeProbe:
    status: int
    missing: str


def parse_envelope_probe(body: bytes) -> tuple[EnvelopeProbe, bool]:
    """Read status and missing from a JSON envelope when present."""
    try:
        env = json.loads(body)
    except json.JSONDecodeError:
        return EnvelopeProbe(0, ""), False
    status = env.get("status")
    data = env.get("data")
    if not isinstance(status, int) or not isinstance(data, list):
        return EnvelopeProbe(0, ""), False
    missing = env.get("missing") or ""
    if not isinstance(missing, str):
        missing = str(missing)
    return EnvelopeProbe(status=status, missing=missing), True


def has_profile_link(header: Any) -> bool:
    """True when the response carries the envelope profile Link header."""
    values: list[str] = []
    if hasattr(header, "get_list"):
        values = list(header.get_list("Link") or header.get_list("link") or [])
    elif hasattr(header, "get_all"):
        values = list(header.get_all("Link") or [])
    for link in values:
        if f"<{PROFILE_URI}>" in link and 'rel="profile"' in link:
            return True
    return False


def match_capability_gap(
    status: int, missing: str,
) -> Callable[[int, Any, bytes], bool]:
    """Matcher for AgentTrader envelope refusals with a given HTTP status and missing capability."""

    def matcher(got: int, header: Any, body: bytes) -> bool:
        if got != status:
            return False
        if is_x402_body(body):
            return False
        probe, ok = parse_envelope_probe(body)
        if not ok or probe.missing != missing:
            return False
        return has_profile_link(header) or probe.status == got

    return matcher
