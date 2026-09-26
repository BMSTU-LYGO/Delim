"""Safe decoding of uploaded receipt images."""

from __future__ import annotations

from io import BytesIO

import cv2
import numpy as np
from PIL import Image, ImageOps, UnidentifiedImageError


MAX_IMAGE_PIXELS = 40_000_000
MAX_IMAGE_DIMENSION = 16_384


class ImageDecodeError(ValueError):
    """Raised when uploaded bytes are not a usable image."""


def decode_image(image_bytes: bytes) -> np.ndarray:
    if not image_bytes:
        raise ImageDecodeError("image is empty")

    try:
        # Pillow performs the decode only once. OpenCV imdecode used to
        # decode the same camera image a second time just to obtain BGR pixels.
        with Image.open(BytesIO(image_bytes)) as source:
            width, height = source.size
            if width <= 0 or height <= 0:
                raise ImageDecodeError("image dimensions are invalid")
            if (
                width > MAX_IMAGE_DIMENSION
                or height > MAX_IMAGE_DIMENSION
                or width * height > MAX_IMAGE_PIXELS
            ):
                raise ImageDecodeError("image resolution exceeds the configured limit")
            oriented = ImageOps.exif_transpose(source)
            rgb = np.asarray(oriented.convert("RGB"))
    except (UnidentifiedImageError, OSError, ValueError) as exc:
        raise ImageDecodeError("image cannot be decoded") from exc

    decoded = cv2.cvtColor(rgb, cv2.COLOR_RGB2BGR)
    if decoded is None or decoded.size == 0:
        raise ImageDecodeError("image cannot be decoded")
    decoded_height, decoded_width = decoded.shape[:2]
    if decoded_width <= 0 or decoded_height <= 0:
        raise ImageDecodeError("image dimensions are invalid")
    return decoded
