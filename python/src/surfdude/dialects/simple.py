from __future__ import annotations

import json
from email.message import Message
from http import HTTPStatus

from surfdude.dialect import HTTPResponse
from surfdude.hypertext import Control, Input, Item, Resource, Schema

MEDIA_TYPE = "application/simple-hypertext+json"


class SimpleDialect:
    def name(self) -> str:
        return "simple"

    def accept(self) -> str:
        return MEDIA_TYPE

    def body_type(self) -> str:
        return "application/json"

    def matches(self, response: HTTPResponse) -> bool:
        mt = _parse_media_type(response.header_get("Content-Type"))
        return mt == MEDIA_TYPE

    def read(self, response: HTTPResponse) -> Resource:
        doc = json.loads(response.body)
        links = doc.get("links") or []
        res = Resource(
            status=response.status_code,
            message=HTTPStatus(response.status_code).phrase,
            controls=_controls(links),
            body=response.body,
        )
        for item in doc.get("items") or []:
            raw = json.dumps(item).encode()
            res.items.append(
                Item(controls=_controls(item.get("links") or []), data=raw)
            )
        return res


def _parse_media_type(value: str) -> str:
    if not value:
        return ""
    msg = Message()
    msg["content-type"] = value
    return msg.get_content_type()


def _controls(links: list) -> list[Control]:
    out: list[Control] = []
    for link in links:
        form = link.get("form") or []
        c = Control(
            rel=link.get("rel", ""),
            title=link.get("description", ""),
            href=link.get("href", ""),
            method=link.get("method", ""),
            if_match=link.get("ifMatch", ""),
            form=len(form) > 0,
        )
        for inp in form:
            hin = Input(name=inp.get("name", ""), required=not inp.get("isOptional", False))
            schema: Schema = {}
            if inp.get("label"):
                schema["description"] = inp["label"]
            opts = inp.get("options")
            if opts:
                schema["enum"] = list(opts)
            if schema:
                hin.schema = schema
            c.inputs.append(hin)
        out.append(c)
    return out
