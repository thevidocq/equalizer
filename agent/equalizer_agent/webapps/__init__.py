"""Web application profiles: one file per kind of application (docs/agent.md).

The crawler, the map building and the verification never name an
application: they ask the profile of the one recognised, or the generic one."""

from . import (drupal, generic, joomla, magento, mediawiki, moodle, opencart, phpbb, prestashop, typo3,
               wordpress)
from .base import ACTIONS, ACTION_WORDS, SENSITIVE, TOKENS, Profile, is_action  # noqa: F401

# The order matters only for recognition by URL when nothing else tells.
ALL = tuple(m.PROFILE for m in (wordpress, joomla, drupal, magento, prestashop, opencart, typo3, moodle,
                                mediawiki, phpbb))
GENERIC = generic.PROFILE
_BY_NAME = {p.name: p for p in ALL + (GENERIC,)}


def get(name: str | None) -> Profile:
    """The profile of a site kind; the generic one when it is unknown."""
    return _BY_NAME.get(name or "", GENERIC)


def names() -> list:
    return [p.name for p in ALL] + [GENERIC.name]
