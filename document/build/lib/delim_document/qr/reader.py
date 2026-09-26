"""QR reader for receipt images."""

from __future__ import annotations

from dataclasses import dataclass

import cv2
import numpy as np


@dataclass(frozen=True, slots=True)
class QRReadResult:
    raw_payload: str | None
    decoded: bool


class ReceiptQRReader:
    def __init__(self) -> None:
        self._detector = cv2.QRCodeDetector()

    def read(
        self,
        original: np.ndarray,
        enhanced: np.ndarray,
        grayscale: np.ndarray | None = None,
    ) -> QRReadResult:
        candidates = (original, enhanced, grayscale)
        for image in candidates:
            if image is None or image.size == 0:
                continue
            payload, _, _ = self._detector.detectAndDecode(image)
            if payload:
                return QRReadResult(raw_payload=payload, decoded=True)
        return QRReadResult(raw_payload=None, decoded=False)
