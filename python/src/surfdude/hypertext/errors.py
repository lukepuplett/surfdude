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

    def __str__(self) -> str:
        text = f"Unable to {self.step}. The server answered {self.status}"
        if self.message:
            text += f": {self.message}"
        if self.resource and self.resource.problems:
            ps = [f"{p.field} {p.message}".strip() for p in self.resource.problems]
            text += f" ({'; '.join(ps)})"
        return text + "."


@dataclass
class NoDataError(Exception):
    def __str__(self) -> str:
        return "Unable to read data. The response carried none."
