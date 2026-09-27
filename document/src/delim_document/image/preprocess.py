"""Conservative receipt preprocessing for OCR."""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass

import cv2
import numpy as np


MIN_SHORT_EDGE = 720
MAX_UPSCALE = 2.5
MAX_DESKEW_DEGREES = 7.0
DESKEW_ANALYSIS_LONG_EDGE = 800
MAX_OCR_LONG_EDGE = 1600

_LOGGER = logging.getLogger("delim_document.ocr")


def _stage(operation: str, started: float) -> None:
    _LOGGER.info(
        "OCR stage completed",
        extra={
            "operation": operation,
            "duration_ms": round((time.monotonic() - started) * 1000),
        },
    )


@dataclass(frozen=True, slots=True)
class PreprocessedReceipt:
    normal: np.ndarray
    grayscale: np.ndarray
    enhanced: np.ndarray


def cap_long_edge(image: np.ndarray, max_long_edge: int = MAX_OCR_LONG_EDGE) -> np.ndarray:
    """Bound camera photos before deskew and OCR work."""
    height, width = image.shape[:2]
    long_edge = max(height, width)
    if long_edge <= max_long_edge:
        return image
    scale = max_long_edge / long_edge
    return cv2.resize(
        image,
        (round(width * scale), round(height * scale)),
        interpolation=cv2.INTER_AREA,
    )


def _resize_for_ocr(image: np.ndarray) -> np.ndarray:
    image = cap_long_edge(image)
    height, width = image.shape[:2]
    short_edge = min(height, width)
    if short_edge >= MIN_SHORT_EDGE:
        return image
    scale = min(
        MAX_UPSCALE,
        MIN_SHORT_EDGE / short_edge,
        MAX_OCR_LONG_EDGE / max(height, width),
    )
    if scale <= 1:
        return image
    return cv2.resize(
        image,
        None,
        fx=scale,
        fy=scale,
        interpolation=cv2.INTER_CUBIC,
    )


def _deskew(image: np.ndarray) -> np.ndarray:
    height, width = image.shape[:2]
    analysis_scale = min(1.0, DESKEW_ANALYSIS_LONG_EDGE / max(height, width))
    analysis = (
        cv2.resize(
            image,
            (round(width * analysis_scale), round(height * analysis_scale)),
            interpolation=cv2.INTER_AREA,
        )
        if analysis_scale < 1.0
        else image
    )
    gray = cv2.cvtColor(analysis, cv2.COLOR_BGR2GRAY)
    _, mask = cv2.threshold(
        gray, 0, 255, cv2.THRESH_BINARY_INV | cv2.THRESH_OTSU
    )
    coordinates = np.column_stack(np.where(mask > 0))
    if len(coordinates) < max(100, mask.size // 100):
        return image

    angle = cv2.minAreaRect(coordinates[:, ::-1].astype(np.float32))[-1]
    angle = 90.0 + angle if angle < -45.0 else angle
    if angle > 45.0:
        angle -= 90.0
    if not 0.5 <= abs(angle) <= MAX_DESKEW_DEGREES:
        return image

    matrix = cv2.getRotationMatrix2D((width / 2, height / 2), angle, 1.0)
    return cv2.warpAffine(
        image,
        matrix,
        (width, height),
        flags=cv2.INTER_CUBIC,
        borderMode=cv2.BORDER_REPLICATE,
    )


def preprocess_receipt(image: np.ndarray) -> PreprocessedReceipt:
    if image.ndim != 3 or image.shape[2] != 3 or image.size == 0:
        raise ValueError("preprocessing expects a non-empty BGR image")

    started = time.monotonic()
    resized = _resize_for_ocr(image)
    _stage("preprocessing_resize", started)

    started = time.monotonic()
    normal = _deskew(resized)
    _stage("preprocessing_deskew", started)

    started = time.monotonic()
    grayscale = cv2.cvtColor(normal, cv2.COLOR_BGR2GRAY)
    _stage("preprocessing_grayscale", started)

    started = time.monotonic()
    enhanced = cv2.createCLAHE(clipLimit=2.0, tileGridSize=(8, 8)).apply(grayscale)
    _stage("preprocessing_contrast", started)
    return PreprocessedReceipt(
        normal=normal,
        grayscale=grayscale,
        enhanced=enhanced,
    )
