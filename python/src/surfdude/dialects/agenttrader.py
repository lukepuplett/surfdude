from __future__ import annotations

import json
from email.message import Message

from surfdude.dialect import HTTPResponse
from surfdude.hypertext import Control, Input, Item, Paging, Problem, Resource

PROFILE_URI = "https://agenttrader.net/profiles/envelope/v1"


class AgentTraderDialect:
    def name(self) -> str:
        return "agenttrader"

    def accept(self) -> str:
        return "application/json"

    def body_type(self) -> str:
        return "application/json"

    def matches(self, response: HTTPResponse) -> bool:
        mt = _parse_media_type(response.header_get("Content-Type"))
        if not (mt == "application/json" or mt.endswith("+json")):
            return False
        for link in response.header_values("Link"):
            if f"<{PROFILE_URI}>" in link and 'rel="profile"' in link:
                return True
        try:
            probe = json.loads(response.body)
        except json.JSONDecodeError:
            return False
        return isinstance(probe.get("status"), int) and isinstance(probe.get("data"), list)

    def read(self, response: HTTPResponse) -> Resource:
        env = json.loads(response.body)
        missing = env.get("missing") or ""
        if not isinstance(missing, str):
            missing = str(missing)
        res = Resource(
            status=env.get("status") or response.status_code,
            message=env.get("message", ""),
            missing=missing,
            notes=list(env.get("notes") or []),
            controls=_to_controls(env.get("_links") or [], env.get("_actions") or []),
            body=response.body,
        )
        paging = env.get("paging")
        if paging:
            res.paging = Paging(
                skip=paging.get("skip", 0),
                take=paging.get("take", 0),
                count=paging.get("count", 0),
                total=paging.get("total"),
            )
        for raw in env.get("data") or []:
            if isinstance(raw, dict):
                item_raw = json.dumps(raw).encode()
                res.items.append(
                    Item(
                        controls=_to_controls(raw.get("_links") or [], raw.get("_actions") or []),
                        notes=list(raw.get("notes") or []),
                        data=item_raw,
                    )
                )
            else:
                res.items.append(Item(data=json.dumps(raw).encode()))
        if res.status >= 400:
            for raw in env.get("data") or []:
                if not isinstance(raw, dict):
                    continue
                if raw.get("message"):
                    res.problems.append(
                        Problem(
                            field=raw.get("field", ""),
                            code=raw.get("code", ""),
                            message=raw.get("message", ""),
                        )
                    )
        return res


def _parse_media_type(value: str) -> str:
    if not value:
        return ""
    msg = Message()
    msg["content-type"] = value
    return msg.get_content_type()


_LINK_CORE = {"rel", "href", "method"}
_ACTION_CORE = {"rel", "action", "method", "enctype", "ifMatch", "fields"}


def _extensions(raw: dict, core: set[str]) -> dict:
    """Every key the model has no field for, as the server sent it."""
    return {k: v for k, v in raw.items() if k not in core}


def _to_controls(links: list, actions: list) -> list[Control]:
    out: list[Control] = []
    for link in links:
        out.append(
            Control(
                rel=link.get("rel", ""),
                href=link.get("href", ""),
                method=link.get("method", ""),
                extensions=_extensions(link, _LINK_CORE),
            )
        )
    for action in actions:
        c = Control(
            rel=action.get("rel", ""),
            href=action.get("action", ""),
            method=action.get("method", ""),
            enctype=action.get("enctype", ""),
            form=True,
            if_match=action.get("ifMatch", ""),
            extensions=_extensions(action, _ACTION_CORE),
        )
        for field in action.get("fields") or []:
            c.inputs.append(
                Input(
                    name=field.get("name", ""),
                    required=field.get("required", False),
                    value=field.get("value"),
                    schema=dict(field.get("schema") or {}),
                )
            )
        out.append(c)
    return out
