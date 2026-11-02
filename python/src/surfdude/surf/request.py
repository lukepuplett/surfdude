from __future__ import annotations

import json
from typing import Any
from urllib.parse import urlencode, urljoin

import httpx

from surfdude.dialect import HTTPResponse, Registry
from surfdude.hypertext import Control
from surfdude.surf.values import expand, nest


def build_request(
    reg: Registry,
    base: str,
    control: Control,
    vals: dict[str, Any],
    token: str,
) -> httpx.Request:
    target, used = expand(control.href, vals)
    url = urljoin(base, target)
    rest = {k: v for k, v in vals.items() if k not in used}
    method = control.http_method()
    headers: dict[str, str] = {"User-Agent": "surfdude-python"}
    content: bytes | None = None

    if method in ("GET", "HEAD", "DELETE"):
        if rest:
            sep = "&" if "?" in url else "?"
            url = f"{url}{sep}{urlencode({k: str(v) for k, v in rest.items()})}"
    elif rest:
        enc = control.enctype
        if not enc:
            first = reg.first()
            enc = first.body_type() if first else ""
        if enc.startswith("application/x-www-form-urlencoded"):
            content = urlencode({k: str(v) for k, v in rest.items()}).encode()
            headers["Content-Type"] = enc
        else:
            content = json.dumps(nest(rest)).encode()
            headers["Content-Type"] = enc or "application/json"

    accept = reg.accept()
    if accept:
        headers["Accept"] = accept
    if control.if_match:
        headers["If-Match"] = control.if_match
    if token:
        headers["Authorization"] = f"Bearer {token}"

    return httpx.Request(method, url, headers=headers, content=content)


def response_from_httpx(resp: httpx.Response) -> HTTPResponse:
    headers: dict[str, list[str]] = {}
    for key, value in resp.headers.multi_items():
        headers.setdefault(key, []).append(value)
    return HTTPResponse(status_code=resp.status_code, headers=headers, body=resp.content)
