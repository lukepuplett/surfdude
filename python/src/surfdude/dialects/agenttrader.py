from __future__ import annotations

import json
from email.message import Message

from surfdude.dialect import HTTPResponse
from surfdude.hypertext import Control, Input, Item, Paging, Price, Problem, Resource

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
        res = Resource(
            status=env.get("status") or response.status_code,
            message=env.get("message", ""),
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


def _to_controls(links: list, actions: list) -> list[Control]:
    out: list[Control] = []
    for link in links:
        out.append(
            Control(
                rel=link.get("rel", ""),
                title=link.get("title", ""),
                href=link.get("href", ""),
                method=link.get("method", ""),
                type=link.get("type", ""),
                requires=list(link.get("requires") or []),
            )
        )
    for action in actions:
        c = Control(
            rel=action.get("rel", ""),
            title=action.get("title", ""),
            href=action.get("action", ""),
            method=action.get("method", ""),
            enctype=action.get("enctype", ""),
            form=True,
            requires=list(action.get("requires") or []),
            if_match=action.get("ifMatch", ""),
        )
        price = action.get("price")
        if price:
            c.price = Price(
                amount=price.get("amount", ""),
                currency=price.get("currency", ""),
                note=price.get("note", ""),
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
