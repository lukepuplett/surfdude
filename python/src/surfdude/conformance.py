from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from surfdude.dialect import Dialect, HTTPResponse
from surfdude.hypertext import Control, Resource


def run(dialect: Dialect, directory: Path) -> None:
    """Run every case in directory/cases.json against dialect (call from pytest)."""
    raw = (directory / "cases.json").read_bytes()
    spec = json.loads(raw)
    if spec["dialect"] != dialect.name():
        raise AssertionError(
            f'cases.json is for dialect {spec["dialect"]!r} but this is {dialect.name()!r}'
        )
    for case in spec["cases"]:
        _run_case(dialect, directory, case)


def _run_case(dialect: Dialect, directory: Path, case: dict[str, Any]) -> None:
    body = (directory / case["file"]).read_bytes()
    headers: dict[str, list[str]] = {}
    for k, v in (case.get("headers") or {}).items():
        headers[k] = [v]
    if case.get("contentType"):
        headers["Content-Type"] = [case["contentType"]]
    response = HTTPResponse(status_code=case.get("status", 200), headers=headers, body=body)
    if not dialect.matches(response):
        raise AssertionError(f"the {dialect.name()} dialect should recognise this response")
    resource = dialect.read(response)
    _check(resource, case["expect"])


def _check(resource: Resource, want: dict[str, Any]) -> None:
    if resource.status != want.get("status", resource.status):
        raise AssertionError(f"status = {resource.status}, want {want.get('status')}")
    if "notes" in want:
        n = want["notes"]
        if n is not None and len(resource.notes) != n:
            raise AssertionError(f"notes = {len(resource.notes)}, want {n}")
    if "paging" in want and want["paging"] is not None:
        got = resource.paging
        wp = want["paging"]
        if (
            got is None
            or got.skip != wp.get("skip", 0)
            or got.take != wp.get("take", 0)
            or got.count != wp.get("count", 0)
            or got.total != wp.get("total")
        ):
            raise AssertionError(f"paging = {got}, want {wp}")
    if "rels" in want and want["rels"] is not None and resource.rels() != want["rels"]:
        raise AssertionError(f"rels = {resource.rels()}, want {want['rels']}")
    for rel, w in (want.get("controls") or {}).items():
        c = resource.control(rel)
        if c is None:
            raise AssertionError(f"missing control {rel!r}")
        _check_control(rel, c, w)
    if "items" in want and want["items"] is not None:
        if len(resource.items) != len(want["items"]):
            raise AssertionError(f"items = {len(resource.items)}, want {len(want['items'])}")
        for i, w in enumerate(want["items"]):
            it = resource.items[i]
            if w.get("rels") is not None and it.rels() != w["rels"]:
                raise AssertionError(f"item {i} rels = {it.rels()}, want {w['rels']}")
            for rel, cw in (w.get("controls") or {}).items():
                c = it.control(rel)
                if c is None:
                    raise AssertionError(f"item {i} is missing control {rel!r}")
                _check_control(rel, c, cw)
    if "problems" in want and want["problems"] is not None:
        if len(resource.problems) != len(want["problems"]):
            raise AssertionError(
                f"problems = {len(resource.problems)}, want {len(want['problems'])}"
            )
        for i, p in enumerate(want["problems"]):
            got = resource.problems[i]
            if got.field != p.get("field") or got.code != p.get("code"):
                raise AssertionError(f"problem {i} = {got}, want {p}")


def _check_control(rel: str, c: Control, w: dict[str, Any]) -> None:
    if c.form != w.get("form", c.form):
        raise AssertionError(f"{rel}: form = {c.form}, want {w.get('form')}")
    if c.http_method() != w.get("method", "GET"):
        raise AssertionError(f"{rel}: method = {c.http_method()}, want {w.get('method')}")
    if w.get("inputs") is not None:
        names = [inp.name for inp in c.inputs]
        if names != w["inputs"]:
            raise AssertionError(f"{rel}: inputs = {names}, want {w['inputs']}")
    if w.get("required") is not None:
        names = [inp.name for inp in c.inputs if inp.required and inp.value is None]
        if names != w["required"]:
            raise AssertionError(f"{rel}: required = {names}, want {w['required']}")
    if w.get("requires") is not None and c.requires != w["requires"]:
        raise AssertionError(f"{rel}: requires = {c.requires}, want {w['requires']}")
    if w.get("ifMatch") is not None and c.if_match != w["ifMatch"]:
        raise AssertionError(f"{rel}: ifMatch = {c.if_match!r}, want {w['ifMatch']!r}")
    if w.get("inputValues"):
        for name, want in w["inputValues"].items():
            inp = next((i for i in c.inputs if i.name == name), None)
            if inp is None:
                raise AssertionError(f"{rel}: input {name!r} not found for inputValues check")
            if not _input_value_equal(inp.value, want):
                raise AssertionError(f"{rel}: input {name!r} value = {inp.value!r}, want {want!r}")


def _input_value_equal(got: Any, want: Any) -> bool:
    if got == want:
        return True
    return str(got) == str(want)
