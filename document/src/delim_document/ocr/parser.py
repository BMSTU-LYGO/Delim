"""Deterministic parsing of normalized receipt OCR lines."""

from __future__ import annotations

from dataclasses import dataclass
import re

from delim_document.ocr.provider import BBox, OCRLine


_SPACE_RE = re.compile(r"\s+")
_MONEY_TOKEN_RE = re.compile(r"(?<!\w)[0-9OО]+[.,][0-9OО]{2}(?!\w)")


@dataclass(frozen=True, slots=True)
class NormalizedLine:
    text: str
    raw_text: str
    confidence: float
    bbox: BBox

    @property
    def left(self) -> float:
        return min((point[0] for point in self.bbox), default=0.0)

    @property
    def top(self) -> float:
        return min((point[1] for point in self.bbox), default=0.0)

    @property
    def right(self) -> float:
        return max((point[0] for point in self.bbox), default=0.0)


def _fix_money_artifacts(text: str) -> str:
    def replace(match: re.Match[str]) -> str:
        return match.group(0).replace("O", "0").replace("О", "0")

    return _MONEY_TOKEN_RE.sub(replace, text)


def normalize_ocr_lines(lines: tuple[OCRLine, ...]) -> tuple[NormalizedLine, ...]:
    normalized: list[tuple[int, NormalizedLine]] = []
    for index, line in enumerate(lines):
        raw = line.text
        text = _fix_money_artifacts(_SPACE_RE.sub(" ", raw.strip()))
        if not text:
            continue
        normalized.append(
            (
                index,
                NormalizedLine(
                    text=text,
                    raw_text=raw,
                    confidence=max(0.0, min(1.0, line.confidence)),
                    bbox=line.bbox,
                ),
            )
        )
    normalized.sort(key=lambda entry: (entry[1].top, entry[1].left, entry[0]))
    return tuple(line for _, line in normalized)
