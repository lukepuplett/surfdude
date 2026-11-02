from __future__ import annotations

from surfdude.hypertext import Control, Price


def preflight(control: Control, token: str) -> None:
    if control.requires and not token:
        raise RequiresNotMetError(rel=control.rel, requires=list(control.requires))
    if control.price is not None:
        raise PaymentRequiredError(rel=control.rel, price=control.price)
    if control.type and "json" not in control.type.lower():
        raise NonHypermediaControlError(rel=control.rel, media_type=control.type)


class RequiresNotMetError(Exception):
    def __init__(self, rel: str, requires: list[str]) -> None:
        self.rel = rel
        self.requires = requires
        super().__init__(
            f'Unable to follow "{rel}". It needs {", ".join(requires)}; '
            "sign in or satisfy that requirement first."
        )


class PaymentRequiredError(Exception):
    def __init__(self, rel: str, price: Price) -> None:
        self.rel = rel
        self.price = price
        note = price.note or "payment"
        super().__init__(f'Unable to submit "{rel}". This control requires payment ({note}).')


class NonHypermediaControlError(Exception):
    def __init__(self, rel: str, media_type: str) -> None:
        self.rel = rel
        self.media_type = media_type
        super().__init__(
            f'Unable to follow "{rel}". It serves {media_type}, '
            "not hypermedia the client can drive."
        )
