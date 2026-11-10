from __future__ import annotations

import json


def is_x402_body(body: bytes) -> bool:
    """True when raw JSON looks like an x402 payment request."""
    try:
        probe = json.loads(body)
    except json.JSONDecodeError:
        return False
    return isinstance(probe.get("x402Version"), int)


def match_x402_payment(status: int, _header: object, body: bytes) -> bool:
    """True for HTTP 402 with x402Version in the body, not an AgentTrader envelope refusal."""
    if status != 402:
        return False
    try:
        probe = json.loads(body)
    except json.JSONDecodeError:
        return False
    return isinstance(probe.get("x402Version"), int)
