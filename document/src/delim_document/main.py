"""Document service entry point."""

from __future__ import annotations

import asyncio
import logging
import signal
import sys

from delim_document.app import App
from delim_document.config import ConfigError, load_config
from delim_document.logging import configure_logging


async def _run(config_path: str) -> None:
    config = load_config(config_path)
    logger = configure_logging(config.app.name, config.app.env)
    stop_event = asyncio.Event()
    loop = asyncio.get_running_loop()
    for signum in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(signum, stop_event.set)
    await App(config, logger).run(stop_event)


def main() -> None:
    config_path = sys.argv[1] if len(sys.argv) > 1 else "configs/document.yaml"
    try:
        asyncio.run(_run(config_path))
    except ConfigError as exc:
        logging.basicConfig(level=logging.ERROR)
        logging.getLogger("document").error("configuration error: %s", exc)
        raise SystemExit(1) from exc
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()
