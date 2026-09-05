"""PaddleOCR production provider."""

from __future__ import annotations

import asyncio
from collections.abc import Iterable
from typing import Any

import numpy as np
from paddleocr import PaddleOCR

from delim_document.ocr.provider import BBox, OCRLine


def _bbox(points: Any, box: Any) -> BBox:
    if points is not None:
        return tuple((float(point[0]), float(point[1])) for point in points)
    if box is not None and len(box) == 4:
        left, top, right, bottom = (float(value) for value in box)
        return ((left, top), (right, top), (right, bottom), (left, bottom))
    return ()


def _extract_lines(results: Iterable[Any]) -> tuple[OCRLine, ...]:
    lines: list[OCRLine] = []
    for result in results:
        texts = result.get("rec_texts", ())
        scores = result.get("rec_scores", ())
        polygons = result.get("rec_polys", ())
        boxes = result.get("rec_boxes", ())
        for index, (text, score) in enumerate(zip(texts, scores, strict=False)):
            cleaned = str(text).strip()
            if not cleaned:
                continue
            polygon = polygons[index] if index < len(polygons) else None
            box = boxes[index] if index < len(boxes) else None
            lines.append(
                OCRLine(
                    text=cleaned,
                    confidence=max(0.0, min(1.0, float(score))),
                    bbox=_bbox(polygon, box),
                )
            )
    return tuple(lines)


class PaddleOCRProvider:
    """One long-lived Russian PP-OCRv5 model."""

    def __init__(
        self, language: str, confidence_threshold: float, concurrency: int = 1
    ) -> None:
        if concurrency != 1:
            raise ValueError("PaddleOCR concurrency must be 1")
        self._model = PaddleOCR(
            lang=language,
            ocr_version="PP-OCRv5",
            device="cpu",
            use_doc_orientation_classify=False,
            use_doc_unwarping=False,
            use_textline_orientation=True,
            text_rec_score_thresh=confidence_threshold,
        )
        self._inference_slots = asyncio.Semaphore(concurrency)

    async def recognize(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        async with self._inference_slots:
            return await asyncio.to_thread(self._recognize_sync, image)

    def _recognize_sync(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        return _extract_lines(self._model.predict(image))
