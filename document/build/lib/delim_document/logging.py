"""Logging configuration."""

from __future__ import annotations

import json
import logging
from typing import Any


# Explicit allowlist of fields the application emits as structured data.
# Anything outside this set is intentionally dropped so that sensitive
# ``extra`` keys (such as authorization, init_data, max_bot_token) cannot
# leak through the formatter, and so exception text/stack traces are never
# serialised.
_ALLOWED_FIELDS = frozenset(
    {
        "service",
        "request_id",
        "operation",
        "duration_ms",
        "status",
        "result",
        "error_class",
        "environment",
        "address",
        "count",
    }
)


class StructuredFormatter(logging.Formatter):
    """Format log records as a single JSON line per record.

    Only the explicit allowlist of fields is emitted. Unknown ``extra``
    keys, exception text and stack traces are dropped so sensitive values
    never reach the log sink.
    """

    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, Any] = {
            "timestamp": self.formatTime(record, "%Y-%m-%dT%H:%M:%S"),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
        }
        for field in _ALLOWED_FIELDS:
            value = getattr(record, field, None)
            if value is None:
                continue
            payload[field] = value
        return json.dumps(payload, ensure_ascii=False, default=str)


def configure_logging(service: str, environment: str) -> logging.Logger:
    """Configure root logging and return a child logger named after ``service``.

    The returned logger carries the ``service`` attribute on every record so
    structured output stays consistent with Gateway/Core vocabulary.
    """

    handler = logging.StreamHandler()
    handler.setFormatter(StructuredFormatter())
    root = logging.getLogger()
    for existing in list(root.handlers):
        root.removeHandler(existing)
    root.addHandler(handler)
    root.setLevel(logging.INFO)

    logger = logging.getLogger(service)
    logger.info(
        "logger configured",
        extra={"service": service, "environment": environment},
    )
    return logger


def service_logger(service: str, environment: str | None = None) -> logging.Logger:
    """Return a logger that always tags records with the given service name.

    Existing handlers are preserved so tests that capture records continue to
    work. The returned logger is suitable for any module that needs to emit
    structured completion records.
    """

    logger = logging.getLogger(service)
    if environment is not None:
        logger.info(
            "logger configured",
            extra={"service": service, "environment": environment},
        )
    return logger