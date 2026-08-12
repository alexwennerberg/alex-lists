from enum import Enum

class Visibility(str, Enum):
    """
    Who can see a mailing list.

    The member names must match the PostgreSQL `visibility` enum in schema.sql,
    and the values must match the strings posted by the visibility radio
    buttons in create.html and settings-info.html.
    """
    PUBLIC = "PUBLIC"
    UNLISTED = "UNLISTED"
    PRIVATE = "PRIVATE"
