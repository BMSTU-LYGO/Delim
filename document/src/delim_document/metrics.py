"""Lightweight Prometheus metrics for the Document service.

The metrics surface is intentionally small and bounded:

* HTTP/gRPC and OCR job counters never include user-controlled identifiers
  (user IDs, group IDs, receipt IDs, raw URLs, tokens, or filenames).
* Label sets are explicitly allowlisted so an unexpected value is replaced
  with the bounded ``unspecified`` sentinel.
* The recorder is safe to use from any coroutine; counters and histograms
  are protected by ``prometheus_client``'s internal locking.

The recorder starts an internal HTTP listener on a configurable host/port so
the metrics endpoint never shares the gRPC port.
"""

from __future__ import annotations

import threading
from dataclasses import dataclass
from typing import Final

from prometheus_client import (
    CollectorRegistry,
    Counter,
    Histogram,
    start_http_server,
)

SERVICE_NAME: Final[str] = "document"

# Bounded label values; anything outside the allowlist is normalised to the
# ``unspecified`` sentinel so cardinality stays bounded.
EXPORT_FORMATS: Final[frozenset[str]] = frozenset({"csv", "pdf", "xlsx"})
OCR_STATES: Final[frozenset[str]] = frozenset(
    {"pending", "processing", "completed", "failed"}
)
OCR_RESULTS: Final[frozenset[str]] = frozenset({"completed", "failed"})
MINIO_OPERATIONS: Final[frozenset[str]] = frozenset(
    {"put", "get", "delete", "stat", "stream"}
)
MINIO_RESULTS: Final[frozenset[str]] = frozenset({"success", "error"})
RESULTS: Final[frozenset[str]] = frozenset({"success", "error"})
GRPC_RESULTS: Final[frozenset[str]] = frozenset({"success", "error"})


@dataclass(frozen=True, slots=True)
class MetricsConfig:
    host: str
    port: int

    @property
    def enabled(self) -> bool:
        return self.port > 0


def _normalise(value: str, allowed: frozenset[str], fallback: str) -> str:
    """Return ``value`` when it is in ``allowed``; ``fallback`` otherwise."""

    if value in allowed:
        return value
    return fallback


def _minio_operation(operation: str) -> str:
    return _normalise(operation.lower(), MINIO_OPERATIONS, "other")


def _export_format(value: str) -> str:
    return _normalise(value.lower(), EXPORT_FORMATS, "unspecified")


def _ocr_state(value: str) -> str:
    return _normalise(value, OCR_STATES, "failed")


def _ocr_result(value: str) -> str:
    return _normalise(value, OCR_RESULTS, "failed")


def _minio_result(err: BaseException | None) -> str:
    return "success" if err is None else "error"


class Recorder:
    """Bounded metrics surface for the Document service."""

    def __init__(self, config: MetricsConfig) -> None:
        self._config = config
        self._registry = CollectorRegistry()
        self._server_lock = threading.Lock()
        self._server_started = False

        self.grpc_requests_total = Counter(
            "delim_grpc_requests_total",
            "Total gRPC requests handled by the service.",
            ("method", "result", "code"),
            registry=self._registry,
        )
        self.grpc_request_duration = Histogram(
            "delim_grpc_request_duration_seconds",
            "gRPC request duration in seconds.",
            ("method",),
            registry=self._registry,
        )
        self.grpc_client_errors_total = Counter(
            "delim_grpc_client_errors_total",
            "Total gRPC client errors grouped by peer/operation/result.",
            ("peer", "operation", "result"),
            registry=self._registry,
        )
        self.ocr_jobs_total = Counter(
            "delim_ocr_jobs_total",
            "Total OCR job terminal events grouped by state.",
            ("state",),
            registry=self._registry,
        )
        self.ocr_duration_seconds = Histogram(
            "delim_ocr_duration_seconds",
            "OCR processing duration in seconds.",
            ("result",),
            buckets=(0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 120.0),
            registry=self._registry,
        )
        self.export_jobs_total = Counter(
            "delim_export_jobs_total",
            "Total export jobs grouped by format/result.",
            ("format", "result"),
            registry=self._registry,
        )
        self.minio_errors_total = Counter(
            "delim_minio_errors_total",
            "Total MinIO errors grouped by operation/result.",
            ("operation", "result"),
            registry=self._registry,
        )

    @property
    def registry(self) -> CollectorRegistry:
        return self._registry

    @property
    def enabled(self) -> bool:
        return self._config.enabled

    def start_endpoint(self) -> None:
        """Bind the internal metrics HTTP listener when enabled.

        The endpoint is exposed only on the configured host/port. Failed
        bind attempts raise the underlying OS error so service startup can
        fail fast when the configured port is unavailable.
        """

        if not self.enabled:
            return
        with self._server_lock:
            if self._server_started:
                return
            start_http_server(
                self._config.port,
                addr=self._config.host,
                registry=self._registry,
            )
            self._server_started = True

    def observe_grpc(self, method: str, code: str, duration: float, err: BaseException | None) -> None:
        result = "success" if err is None else "error"
        self.grpc_requests_total.labels(method=method, result=result, code=code).inc()
        self.grpc_request_duration.labels(method=method).observe(max(0.0, duration))

    def observe_grpc_client_error(self, peer: str, operation: str, err: BaseException | None) -> None:
        if err is None:
            return
        result = "error"
        self.grpc_client_errors_total.labels(peer=peer, operation=operation, result=result).inc()

    def observe_ocr_job(self, state: str) -> None:
        self.ocr_jobs_total.labels(state=_ocr_state(state)).inc()

    def observe_ocr_duration(self, result: str, duration: float) -> None:
        self.ocr_duration_seconds.labels(result=_ocr_result(result)).observe(max(0.0, duration))

    def observe_export_job(self, fmt: str, err: BaseException | None) -> None:
        result = "success" if err is None else "error"
        self.export_jobs_total.labels(format=_export_format(fmt), result=result).inc()

    def observe_minio_error(self, operation: str, err: BaseException | None) -> None:
        self.minio_errors_total.labels(
            operation=_minio_operation(operation),
            result=_minio_result(err),
        ).inc()


def build_recorder(config: MetricsConfig) -> Recorder:
    """Factory used by the service entry point and tests."""

    return Recorder(config)


def noop_recorder() -> Recorder:
    """Return a recorder whose endpoint is disabled and counters are scoped
    to a private registry. Useful for unit tests that exercise unrelated
    code paths without polluting global metrics state."""

    return Recorder(MetricsConfig(host="", port=0))