"""Safe decoding of uploaded receipt images."""

from __future__ import annotations

from io import BytesIO

import cv2
import numpy as np
from PIL import Image, UnidentifiedImageError


MAX_IMAGE_PIXELS = 40_000_000
MAX_IMAGE_DIMENSION = 16_384


class ImageDecodeError(ValueError):
    """Raised when uploaded bytes are not a usable image."""


def decode_image(image_bytes: bytes) -> np.ndarray:
    if not image_bytes:
        raise ImageDecodeError("image is empty")

    try:
        with Image.open(BytesIO(image_bytes)) as probe:
            width, height = probe.size
    except (UnidentifiedImageError, OSError, ValueError) as exc:
        raise ImageDecodeError("image cannot be decoded") from exc

    if width <= 0 or height <= 0:
        raise ImageDecodeError("image dimensions are invalid")
    if (
        width > MAX_IMAGE_DIMENSION
        or height > MAX_IMAGE_DIMENSION
        or width * height > MAX_IMAGE_PIXELS
    ):
        raise ImageDecodeError("image resolution exceeds the configured limit")

    encoded = np.frombuffer(image_bytes, dtype=np.uint8)
    decoded = cv2.imdecode(encoded, cv2.IMREAD_COLOR)
    if decoded is None or decoded.size == 0:
        raise ImageDecodeError("image cannot be decoded")
    decoded_height, decoded_width = decoded.shape[:2]
    if decoded_width <= 0 or decoded_height <= 0:
        raise ImageDecodeError("image dimensions are invalid")
    return decoded
