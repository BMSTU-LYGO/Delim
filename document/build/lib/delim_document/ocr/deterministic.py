"""Deterministic OCR provider for E2E (test-only, Block 10.1).

This provider is wired ONLY when the Document service runs with ``app.env ==
"test"``. It replaces just the Paddle inference step: the rest of the real
pipeline (Gateway -> gRPC -> CreateReceipt -> Job -> worker -> OCRResult) runs
unchanged. Production never selects this provider, and there is no automatic
fallback from the production Paddle provider to a fake.

It returns a fixed, synthetic Russian receipt (no personal data) so browser E2E
can exercise the OCR review flow on any CPU architecture.
"""

from __future__ import annotations

import numpy as np

from delim_document.ocr.provider import OCRLine

# Synthetic receipt lines. Values are chosen to exercise item parsing, total,
# date and service-line exclusion (ИТОГО/НДС/ФН must never become items).
_FIXED_LINES: tuple[tuple[str, float, float], ...] = (
    ("КАССОВЫЙ ЧЕК", 0.95, 20),
    ("2025-01-02 03:04", 0.97, 40),
    ("Хлеб белый 1x25.00 25.00", 0.96, 80),
    ("Молоко 3.2% 1x17.50 17.50", 0.94, 100),
    ("НДС 10% 3.86", 0.90, 120),
    ("ИТОГО 42.50", 0.98, 140),
    ("ФН 9250440300", 0.88, 160),
)


class DeterministicOCRProvider:
    """Returns a fixed receipt for every image; used only in test environments."""

    def __init__(self, language: str, confidence_threshold: float, concurrency: int = 1) -> None:
        del language, confidence_threshold, concurrency

    @property
    def degraded(self) -> bool:
        return False

    def status(self) -> str:
        # The deterministic provider always works in the test environment.
        return "ok"

    async def recognize(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        del image
        return tuple(
            OCRLine(text=text, confidence=confidence, bbox=((40, y), (560, y), (560, y + 24), (40, y + 24)))
            for text, confidence, y in _FIXED_LINES
        )

    def _recognize_sync(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        del image
        return tuple(
            OCRLine(text=text, confidence=confidence, bbox=((40, y), (560, y), (560, y + 24), (40, y + 24)))
            for text, confidence, y in _FIXED_LINES
        )


def build_provider(language: str, confidence_threshold: float) -> DeterministicOCRProvider:
    return DeterministicOCRProvider(language, confidence_threshold, 1)