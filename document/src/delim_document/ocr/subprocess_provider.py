"""Subprocess-isolated OCR inference (Block 2.3).

The native Paddle runtime can crash the whole interpreter (unrecoverable SIGSEGV
on CPU builds for some platforms, e.g. linux/aarch64). This provider runs OCR
inference in a separate OS process (``delim_document.ocr.ocr_worker``) over a
binary pipe protocol, so a native crash takes down only the worker, never the
Document gRPC / database / MinIO main process.

Behaviour:
  * The child loads the real PaddleOCR model once and answers image requests.
  * If the child dies (segfault/killed), times out, or fails to start, the call
    raises :class:`OCRWorkerUnavailableError`; the OCR worker maps that to a
    normal retry and eventually a ``failed`` job.
  * The next call respawns the child transparently.
  * ``status()`` reports the OCR subsystem state for readiness (Block 2.4):
    ``ok`` | ``degraded`` | ``unknown``. No silent fallback to a fake OCR.

It is deliberately an OS subprocess, not a separate microservice.
"""

from __future__ import annotations

import os
import pickle
import struct
import subprocess
import sys
import threading
from typing import Any

import numpy as np

from delim_document.ocr.provider import OCRLine

_READY = b"R"
_INIT_ERROR = b"E"
_OK = b"K"
_INFER_ERROR = b"F"


class OCRWorkerUnavailableError(RuntimeError):
    """The isolated OCR inference process is unavailable (crash/timeout/startup failure)."""


def _read_exactly(stream, size: int) -> bytes | None:
    buffer = bytearray()
    while len(buffer) < size:
        chunk = stream.read(size - len(buffer))
        if not chunk:
            return None
        buffer.extend(chunk)
    return bytes(buffer)


class SubprocessOCRProvider:
    def __init__(
        self,
        language: str,
        confidence_threshold: float,
        request_timeout: float = 120.0,
        startup_timeout: float = 180.0,
    ) -> None:
        self._language = language
        self._threshold = confidence_threshold
        self._request_timeout = request_timeout
        self._startup_timeout = startup_timeout
        self._lock = threading.Lock()
        self._proc: subprocess.Popen | None = None
        self._rid = 0
        self._started_once = False
        self._degraded = False
        self._last_error: str | None = None

    # -- lifecycle ---------------------------------------------------------
    def _alive(self) -> bool:
        return self._proc is not None and self._proc.poll() is None

    def _shutdown(self) -> None:
        proc = self._proc
        self._proc = None
        if proc is None or proc.poll() is not None:
            return
        try:
            proc.stdin.write(struct.pack(">I", 0))  # shutdown frame
            proc.stdin.flush()
        except (BrokenPipeError, OSError, ValueError):
            pass
        try:
            proc.wait(timeout=3)
        except subprocess.TimeoutExpired:
            proc.kill()
            with _suppressed():
                proc.wait(timeout=2)

    def _start(self) -> None:
        self._shutdown()
        env = dict(os.environ)
        env["DELIM_OCR_LANGUAGE"] = self._language
        env["DELIM_OCR_THRESHOLD"] = str(self._threshold)
        try:
            proc = subprocess.Popen(
                [sys.executable, "-m", "delim_document.ocr.ocr_worker"],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.DEVNULL,
                env=env,
            )
        except OSError as exc:
            self._fail(f"spawn failed: {exc!r}")
            raise OCRWorkerUnavailableError(self._last_error or "spawn failed") from exc

        self._proc = proc
        frame = _read_exactly(proc.stdout, 5)
        if frame is None:
            code = proc.wait(timeout=5) if proc.poll() is None else proc.poll()
            self._fail(f"worker exited during startup (exit={code})")
            raise OCRWorkerUnavailableError(self._last_error or "startup EOF")
        kind, length = frame[:1], struct.unpack(">I", frame[1:])[0]
        payload = _read_exactly(proc.stdout, length) if length else b""
        if kind == _INIT_ERROR:
            self._fail(f"model init failed: {payload!r}")
            raise OCRWorkerUnavailableError(self._last_error or "model init failed")
        if kind != _READY:
            self._fail(f"unexpected startup frame: {kind!r}")
            raise OCRWorkerUnavailableError(self._last_error or "startup protocol error")
        self._degraded = False
        self._last_error = None
        self._started_once = True

    def _fail(self, reason: str) -> None:
        self._last_error = reason
        self._degraded = True
        self._shutdown()

    # -- OCRProvider surface (async) --------------------------------------
    async def recognize(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        import asyncio

        return await asyncio.to_thread(self.recognize_sync, image)

    # -- OCR request/response over the pipe --------------------------------
    def recognize_sync(self, image: np.ndarray) -> tuple[OCRLine, ...]:
        with self._lock:
            if not self._alive():
                self._start()
            self._rid += 1
            rid = self._rid
            request = pickle.dumps((rid, image))
            try:
                self._proc.stdin.write(struct.pack(">I", len(request)) + request)
                self._proc.stdin.flush()
            except (BrokenPipeError, OSError, ValueError, AttributeError) as exc:
                self._fail(f"send failed: {exc!r}")
                raise OCRWorkerUnavailableError(self._last_error or "send failed") from exc
            return self._await_response(rid)

    def _await_response(self, rid: int) -> tuple[OCRLine, ...]:
        proc = self._proc
        watchdog = threading.Timer(self._request_timeout, self._kill_stalled)
        watchdog.daemon = True
        watchdog.start()
        try:
            while True:
                header = _read_exactly(proc.stdout, 5)
                if header is None:
                    code = proc.poll()
                    self._fail(f"worker died during inference (exit={code})")
                    raise OCRWorkerUnavailableError(self._last_error or "worker died")
                kind, length = header[:1], struct.unpack(">I", header[1:])[0]
                payload = _read_exactly(proc.stdout, length) if length else b""
                if payload is None:
                    self._fail("truncated response frame")
                    raise OCRWorkerUnavailableError(self._last_error or "truncated response")
                if kind == _OK:
                    data = pickle.loads(payload)
                    return tuple(
                        OCRLine(text=text, confidence=conf, bbox=tuple(map(tuple, box)))
                        for text, conf, box in data
                    )
                if kind == _INFER_ERROR:
                    err_rid, detail = pickle.loads(payload)
                    if err_rid == rid:
                        # Python-level model error: fail the job, keep the child.
                        raise OCRWorkerUnavailableError(f"OCR inference error: {detail}")
                # unknown/foreign frame -> keep reading
        finally:
            watchdog.cancel()

    def _kill_stalled(self) -> None:
        proc = self._proc
        if proc is not None and proc.poll() is None:
            proc.kill()

    # -- health (Block 2.4) ------------------------------------------------
    def status(self) -> str:
        if self._degraded:
            return "degraded"
        if self._started_once and self._alive():
            return "ok"
        return "unknown"

    @property
    def degraded(self) -> bool:
        return self._degraded

    @property
    def last_error(self) -> str | None:
        return self._last_error

    def health(self) -> dict[str, Any]:
        return {"worker_alive": self._alive(), "degraded": self._degraded, "last_error": self._last_error}

    def close(self) -> None:
        with self._lock:
            self._shutdown()


class _suppressed:
    def __enter__(self):
        return self

    def __exit__(self, *exc: Any) -> bool:
        return True
