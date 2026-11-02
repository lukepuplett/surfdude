"""Merge prefilled input values from hypermedia with journey values."""

from __future__ import annotations

from surfdude.hypertext import Control, Input
from surfdude.surf.values import merge


def test_prefilled_input_is_sent_unless_overridden() -> None:
    control = Control(
        rel="save",
        form=True,
        inputs=[
            Input(name="sessionToken", value="tok_abc"),
            Input(name="title", required=True),
        ],
    )

    out = merge(control, {"title": "Hello"})
    assert out["sessionToken"] == "tok_abc"
    assert out["title"] == "Hello"

    out = merge(control, {"sessionToken": "override", "title": "Hello"})
    assert out["sessionToken"] == "override"


def test_prefill_satisfies_required_without_caller_value() -> None:
    control = Control(
        rel="shortlist",
        inputs=[
            Input(name="listingId", required=True, value="lst_1"),
            Input(name="note"),
        ],
    )

    out = merge(control, {"note": "nice"})
    assert out["listingId"] == "lst_1"
