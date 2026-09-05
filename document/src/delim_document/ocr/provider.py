"""OCR provider abstraction for the processing worker."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal
from typing import Protocol

import numpy as np


BBox = tuple[tuple[float, float], ...]


@dataclass(frozen=True, slots=True)
class OCRLine:
    text: str
    confidence: float
    bbox: BBox


@dataclass(frozen=True, slots=True)
class OCRItem:
    name: str
    quantity: Decimal | None
    unit_price_minor: int | None
    amount_minor: int
    confidence: float


@dataclass(frozen=True, slots=True)
class OCRResult:
    merchant: str | None
    merchant_confidence: float
    date: datetime | None
    date_confidence: float
    total_minor: int | None
    total_confidence: float
    currency: str | None
    items: tuple[OCRItem, ...]
    confidence: float
    raw_text: str
    raw_lines: tuple[OCRLine, ...]
    qr_raw: str | None
    total_mismatch: bool


class OCRProvider(Protocol):
    async def recognize(self, image: np.ndarray) -> tuple[OCRLine, ...]: ...


class OCRNotImplementedError(NotImplementedError):
    code = "not_implemented"


class StubOCRProvider:
    async def recognize(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        del image
        raise OCRNotImplementedError("not_implemented")
