"""Conservative receipt preprocessing for OCR."""

from __future__ import annotations

from dataclasses import dataclass

import cv2
import numpy as np


MIN_SHORT_EDGE = 900
MAX_UPSCALE = 2.5
MAX_DESKEW_DEGREES = 7.0


@dataclass(frozen=True, slots=True)
class PreprocessedReceipt:
    normal: np.ndarray
    grayscale: np.ndarray
    enhanced: np.ndarray
    binarized: np.ndarray


def _resize_small(image: np.ndarray) -> np.ndarray:
    height, width = image.shape[:2]
    short_edge = min(height, width)
    if short_edge >= MIN_SHORT_EDGE:
        return image.copy()
    scale = min(MAX_UPSCALE, MIN_SHORT_EDGE / short_edge)
    return cv2.resize(
        image,
        None,
        fx=scale,
        fy=scale,
        interpolation=cv2.INTER_CUBIC,
    )


def _deskew(image: np.ndarray) -> np.ndarray:
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
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

    height, width = image.shape[:2]
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

    normal = _deskew(_resize_small(image))
    grayscale = cv2.cvtColor(normal, cv2.COLOR_BGR2GRAY)
    denoised = cv2.fastNlMeansDenoising(grayscale, None, 5, 7, 21)
    enhanced = cv2.createCLAHE(clipLimit=2.0, tileGridSize=(8, 8)).apply(denoised)
    binarized = cv2.adaptiveThreshold(
        enhanced,
        255,
        cv2.ADAPTIVE_THRESH_GAUSSIAN_C,
        cv2.THRESH_BINARY,
        31,
        9,
    )
    return PreprocessedReceipt(
        normal=normal,
        grayscale=grayscale,
        enhanced=enhanced,
        binarized=binarized,
    )
