"""Document service use cases."""

from __future__ import annotations

from delim_document.config import UploadConfig


ALLOWED_CONTENT_TYPES = frozenset({"image/jpeg", "image/png", "image/webp"})


class InvalidInputError(ValueError):
    """Raised when receipt input is not valid."""


def validate_receipt_upload(
    content: bytes, content_type: str, config: UploadConfig
) -> None:
    if not content:
        raise InvalidInputError("receipt image is empty")
    if len(content) > config.max_size_bytes:
        raise InvalidInputError("receipt image exceeds the configured size limit")
    if content_type not in ALLOWED_CONTENT_TYPES:
        raise InvalidInputError("unsupported receipt content type")

    detected_type: str | None = None
    if len(content) >= 3 and content[:3] == b"\xff\xd8\xff":
        detected_type = "image/jpeg"
    elif len(content) >= 8 and content[:8] == b"\x89PNG\r\n\x1a\n":
        detected_type = "image/png"
    elif (
        len(content) >= 12
        and content[:4] == b"RIFF"
        and content[8:12] == b"WEBP"
    ):
        detected_type = "image/webp"

    if detected_type != content_type:
        raise InvalidInputError("receipt bytes do not match content type")
