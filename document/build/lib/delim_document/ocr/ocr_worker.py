"""Child process that hosts the native PaddleOCR model (Block 2.3).

Run as: ``python -m delim_document.ocr.ocr_worker``

Protocol (over stdin/stdout, binary):
  * Child -> parent frames: [type:1][length:4 big-endian][payload]
      'R' ready (empty)            model loaded
      'E' init error               payload = utf-8 error text
      'K' ok result                payload = pickle(list[(text, conf, bbox)])
      'F' inference error          payload = pickle((rid:int, text:str))
  * Parent -> child frames: [length:4 big-endian][payload]
      payload = pickle((rid:int, image:np.ndarray)); length 0 -> shutdown.

A native SIGSEGV kills only this process; the parent observes EOF and fails the
job (retry -> failed) without affecting the Document gRPC service.
"""

from __future__ import annotations

import os
import pickle
import struct
import sys


def _write_frame(stream, kind: bytes, payload: bytes) -> None:
    stream.write(kind + struct.pack(">I", len(payload)) + payload)
    stream.flush()


def _read_frame(stream):
    header = _read_exactly(stream, 5)
    if header is None:
        return None, None
    kind = header[:1]
    (length,) = struct.unpack(">I", header[1:])
    payload = _read_exactly(stream, length) if length else b""
    if payload is None:
        return None, None
    return kind, payload


def _read_exactly(stream, size: int):
    buffer = bytearray()
    while len(buffer) < size:
        chunk = stream.read(size - len(buffer))
        if not chunk:
            return None
        buffer.extend(chunk)
    return bytes(buffer)


def main() -> int:
    out = sys.stdout.buffer
    stdin = sys.stdin.buffer
    try:
        from delim_document.ocr.paddle import PaddleOCRProvider

        language = os.environ.get("DELIM_OCR_LANGUAGE", "ru")
        threshold = float(os.environ.get("DELIM_OCR_THRESHOLD", "0.45"))
        provider = PaddleOCRProvider(language, threshold, 1)
    except BaseException as exc:  # noqa: BLE001
        _write_frame(out, b"E", repr(exc).encode("utf-8", "replace"))
        return 1

    _write_frame(out, b"R", b"")
    while True:
        header = _read_exactly(stdin, 4)
        if header is None:
            return 0  # parent closed stdin
        (length,) = struct.unpack(">I", header)
        if length == 0:
            return 0
        payload = _read_exactly(stdin, length)
        if payload is None:
            return 0
        try:
            rid, image = pickle.loads(payload)
        except BaseException as exc:  # noqa: BLE001
            _write_frame(out, b"F", pickle.dumps((0, f"bad request: {exc!r}")))
            continue
        try:
            lines = provider._recognize_sync(image)  # noqa: SLF001 - internal worker
            data = [(line.text, line.confidence, list(line.bbox)) for line in lines]
            _write_frame(out, b"K", pickle.dumps(data))
        except BaseException as exc:  # noqa: BLE001 - stay alive on python errors
            _write_frame(out, b"F", pickle.dumps((rid, repr(exc))))


if __name__ == "__main__":
    sys.exit(main())
