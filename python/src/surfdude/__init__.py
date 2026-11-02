"""Public exports for the surfdude package."""

from surfdude.dialects import AgentTraderDialect, SimpleDialect
from surfdude.surf.surfer import Page, Surfer
from surfdude.surf.wave import Report, Wave

__all__ = [
    "AgentTraderDialect",
    "Page",
    "Report",
    "SimpleDialect",
    "Surfer",
    "Wave",
]
