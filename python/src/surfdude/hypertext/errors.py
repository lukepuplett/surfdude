from __future__ import annotations

from dataclasses import dataclass

from surfdude.hypertext.model import Input, Resource


@dataclass
class RelNotFoundError(Exception):
    rel: str
    where: str
    offered: list[str]

    def __str__(self) -> str:
        offered = ", ".join(self.offered) if self.offered else "nothing"
        return (
            f'Unable to follow "{self.rel}". {self.where} has no control with that rel. '
            f"It offers: {offered}."
        )


@dataclass
class MissingInputError(Exception):
    rel: str
    missing: list[Input]

    def __str__(self) -> str:
        parts: list[str] = []
        for inp in self.missing:
            label = inp.name
            desc = inp.schema.get("description") if inp.schema else None
            if isinstance(desc, str) and desc:
                label = f"{inp.name} ({desc})"
            parts.append(label)
        return f'Unable to submit "{self.rel}". It needs: {", ".join(parts)}.'


@dataclass
class InvalidInputError(Exception):
    rel: str
    name: str
    reason: str

    def __str__(self) -> str:
        return (
            f'Unable to submit "{self.rel}". The value for {self.name} is not allowed: {self.reason}.'
        )


@dataclass
class UnexpectedInputError(Exception):
    rel: str
    name: str
    allowed: list[str]

    def __str__(self) -> str:
        return (
            f'Unable to submit "{self.rel}". It has no input called {self.name}. '
            f"Its inputs are: {', '.join(self.allowed)}."
        )


@dataclass
class UnsupportedMediaTypeError(Exception):
    content_type: str
    tried: list[str]

    def __str__(self) -> str:
        return (
            f'Unable to read the response. No dialect understands "{self.content_type}". '
            f"Dialects tried: {', '.join(self.tried)}."
        )


@dataclass
class UnexpectedStatusError(Exception):
    step: str
    status: int
    message: str = ""
    resource: Resource | None = None
    attempts: int = 1
    """How many times the request was sent. More than 1 only when a retry policy tried again."""
    url: str = ""

    def __str__(self) -> str:
        text = f"Unable to {self.step}. The server answered {self.status}"
        if self.message:
            text += f": {self.message}"
        if self.resource and self.resource.problems:
            ps = [f"{p.field} {p.message}".strip() for p in self.resource.problems]
            text += f" ({'; '.join(ps)})"
        text += "."
        if self.attempts > 1:
            text += f" It was tried {self.attempts} times."
        return text


@dataclass
class RequestFailedError(Exception):
    """The request never got an answer, for example the connection failed or timed out."""

    step: str
    cause: Exception
    attempts: int = 1

    def __str__(self) -> str:
        tried = f" after {self.attempts} attempts" if self.attempts > 1 else ""
        return f"Unable to {self.step}. The request failed{tried}: {self.cause}"


@dataclass
class SignInFailedError(Exception):
    """Signing in again failed: the hook raised, or it gave no token."""

    step: str
    cause: Exception | None = None

    def __str__(self) -> str:
        if self.cause is None:
            return (
                f"Unable to {self.step}. Signing in again gave no token, "
                "so the request cannot be sent again."
            )
        return f"Unable to {self.step}. Signing in again failed: {self.cause}"


@dataclass
class NoDataError(Exception):
    def __str__(self) -> str:
        return "Unable to read data. The response carried none."


class MultiItemReadError(Exception):
    def __init__(self, count: int) -> None:
        self.count = count
        super().__init__(
            f"Unable to read data. This page has {count} items; select one with Item(i)."
        )
