"""Logging configuration."""

import logging


def configure_logging(service: str, environment: str) -> logging.Logger:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    logger = logging.getLogger(service)
    logger.info("logger configured", extra={"environment": environment})
    return logger
