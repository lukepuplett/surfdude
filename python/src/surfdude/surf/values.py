from __future__ import annotations

import re
from typing import Any
from urllib.parse import quote, urlencode

from surfdude.hypertext import (
    Control,
    Input,
    InvalidInputError,
    MissingInputError,
    UnexpectedInputError,
)

Values = dict[str, Any]

_TEMPLATE = re.compile(r"\{(\?)?([^}]+)\}")


def merge(control: Control, values: Values | None) -> dict[str, Any]:
    v = values or {}
    allowed: dict[str, bool] = {inp.name: True for inp in control.inputs}
    for name in template_vars(control.href):
        allowed[name] = True
    for name in v:
        if name not in allowed:
            raise UnexpectedInputError(
                rel=control.rel,
                name=name,
                allowed=sorted(allowed.keys()),
            )
    out: dict[str, Any] = {}
    for inp in control.inputs:
        if inp.value is not None:
            out[inp.name] = inp.value
    out.update(v)
    missing: list[Input] = []
    for inp in control.inputs:
        if inp.required and inp.name not in out:
            missing.append(inp)
    if missing:
        raise MissingInputError(rel=control.rel, missing=missing)
    for inp in control.inputs:
        if inp.name in out:
            reason = check_schema(inp.schema, out[inp.name])
            if reason:
                raise InvalidInputError(rel=control.rel, name=inp.name, reason=reason)
    return out


def check_schema(schema: dict[str, Any], val: Any) -> str:
    if not schema:
        return ""
    t = schema.get("type")
    if isinstance(t, str):
        reason = _check_type(t, val)
        if reason:
            return reason
    enum = schema.get("enum")
    if isinstance(enum, list) and enum and not any(same_value(e, val) for e in enum):
        return f"must be one of {', '.join(str(e) for e in enum)}"
    n, ok = to_float(val)
    if ok:
        min_v, max_v = schema.get("minimum"), schema.get("maximum")
        if min_v is not None:
            mf, _ = to_float(min_v)
            if mf is not None and n < mf:
                return f"must be at least {min_v}"
        if max_v is not None:
            xf, _ = to_float(max_v)
            if xf is not None and n > xf:
                return f"must be at most {max_v}"
    if isinstance(val, str):
        length = len(val)
        if schema.get("minLength") is not None:
            mf, _ = to_float(schema["minLength"])
            if mf is not None and length < mf:
                return f"must be at least {schema['minLength']} characters"
        if schema.get("maxLength") is not None:
            xf, _ = to_float(schema["maxLength"])
            if xf is not None and length > xf:
                return f"must be at most {schema['maxLength']} characters"
        pattern = schema.get("pattern")
        if isinstance(pattern, str) and re.fullmatch(pattern, val) is None:
            return f"must match {pattern}"
    return ""


def _check_type(t: str, val: Any) -> str:
    if t == "string" and not isinstance(val, str):
        return "must be text"
    if t == "integer":
        f, ok = to_float(val)
        if not ok or f != int(f):
            return "must be a whole number"
    if t == "number" and not to_float(val)[1]:
        return "must be a number"
    if t == "boolean" and not isinstance(val, bool):
        return "must be true or false"
    return ""


def to_float(val: Any) -> tuple[float, bool]:
    if isinstance(val, bool):
        return float(val), True
    if isinstance(val, int):
        return float(val), True
    if isinstance(val, float):
        return val, True
    return 0.0, False


def same_value(a: Any, b: Any) -> bool:
    fa, oka = to_float(a)
    fb, okb = to_float(b)
    if oka and okb:
        return fa == fb
    return str(a) == str(b)


def nest(flat: dict[str, Any]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for key, val in flat.items():
        parts = key.split(".")
        cur = out
        for i, part in enumerate(parts):
            if i == len(parts) - 1:
                cur[part] = val
                break
            nxt = cur.get(part)
            if not isinstance(nxt, dict):
                nxt = {}
                cur[part] = nxt
            cur = nxt
    return out


def template_vars(href: str) -> list[str]:
    out: list[str] = []
    for m in _TEMPLATE.finditer(href):
        _optional, names = m.group(1), m.group(2)
        for name in names.split(","):
            out.append(name.strip())
    return out


def expand(href: str, vals: dict[str, Any]) -> tuple[str, dict[str, bool]]:
    used: dict[str, bool] = {}

    def repl(match: re.Match[str]) -> str:
        is_query, names_blob = match.group(1), match.group(2)
        names = [n.strip() for n in names_blob.split(",")]
        if is_query == "?":
            q: dict[str, str] = {}
            for name in names:
                if name in vals:
                    used[name] = True
                    q[name] = str(vals[name])
            if not q:
                return ""
            return "?" + urlencode(q)
        name = names[0]
        if name in vals:
            used[name] = True
            return quote(str(vals[name]), safe="")
        return match.group(0)

    return _TEMPLATE.sub(repl, href), used
