"""Small async client for the third-party ProverkaCheka receipt API.

This module deliberately does not call itself an FNS client: it sends a QR
image to ``proverkacheka.com``, which is an external service.  Failures are
reported as typed exceptions so callers can safely fall back to local OCR.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
import json
from typing import Awaitable, Callable, Mapping, Protocol
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen
from uuid import uuid4


DEFAULT_ENDPOINT = "https://proverkacheka.com/api/v1/check/get"


@dataclass(frozen=True, slots=True)
class HttpResponse:
    """The bounded portion of an HTTP response returned by a transport."""

    status: int
    body: bytes


@dataclass(frozen=True, slots=True)
class ReceiptItem:
    name: str
    price_minor: int
    quantity: Decimal
    total_minor: int


@dataclass(frozen=True, slots=True)
class ReceiptLookupResult:
    total_minor: int | None
    seller_name: str | None
    seller_inn: str | None
    retail_place_address: str | None
    ticket_date: str | None
    items: tuple[ReceiptItem, ...]


class ReceiptLookupError(RuntimeError):
    """Base error for a receipt lookup that should fall back to OCR."""


class ReceiptLookupTransportError(ReceiptLookupError):
    """The third-party service could not be reached safely."""


class ReceiptLookupResponseError(ReceiptLookupError):
    """The third-party service returned an invalid or unusable response."""


class ReceiptLookupExhaustedError(ReceiptLookupError):
    """The service did not finish processing before all attempts were used."""


class AsyncTransport(Protocol):
    def __call__(
        self,
        url: str,
        body: bytes,
        headers: Mapping[str, str],
        timeout_seconds: float,
        max_response_bytes: int,
    ) -> Awaitable[HttpResponse]: ...


AsyncSleep = Callable[[float], Awaitable[None]]


class ProverkaChekaClient:
    """Look up receipt positions by uploading the fiscal QR image.

    ``lookup`` is a coroutine. Cancelling its task cancels waits immediately;
    a running stdlib HTTP request cannot be force-cancelled but its timeout is
    bounded and its result is discarded after cancellation.
    """

    def __init__(
        self,
        token: str,
        *,
        endpoint: str = DEFAULT_ENDPOINT,
        timeout_seconds: float = 10.0,
        max_response_bytes: int = 1_048_576,
        max_attempts: int = 5,
        retry_delay_code_2_seconds: float = 2.0,
        retry_delay_code_4_seconds: float = 8.0,
        transport: AsyncTransport | None = None,
        sleep: AsyncSleep = asyncio.sleep,
    ) -> None:
        if not token.strip():
            raise ValueError("ProverkaCheka token must not be empty")
        if timeout_seconds <= 0:
            raise ValueError("timeout_seconds must be positive")
        if max_response_bytes <= 0:
            raise ValueError("max_response_bytes must be positive")
        if max_attempts <= 0:
            raise ValueError("max_attempts must be positive")
        if retry_delay_code_2_seconds < 0 or retry_delay_code_4_seconds < 0:
            raise ValueError("retry delays must not be negative")

        self._token = token
        self._endpoint = endpoint
        self._timeout_seconds = timeout_seconds
        self._max_response_bytes = max_response_bytes
        self._max_attempts = max_attempts
        self._retry_delays = {
            2: retry_delay_code_2_seconds,
            4: retry_delay_code_4_seconds,
        }
        self._transport = transport or _urllib_transport
        self._sleep = sleep

    async def lookup(self, image_bytes: bytes) -> ReceiptLookupResult:
        """Return receipt line items or raise a typed :class:`ReceiptLookupError`."""
        if not image_bytes:
            raise ReceiptLookupResponseError("QR image must not be empty")

        body, content_type = _encode_multipart(self._token, image_bytes)
        headers = {"Content-Type": content_type, "Accept": "application/json"}

        for attempt in range(1, self._max_attempts + 1):
            response = await self._send(body, headers)
            payload = _parse_api_response(response)
            code = payload["code"]
            if code == 1:
                return _parse_receipt(payload["data"])
            if code not in self._retry_delays:
                raise ReceiptLookupResponseError(f"ProverkaCheka returned code {code}")
            if attempt == self._max_attempts:
                break
            await self._sleep(self._retry_delays[code])

        raise ReceiptLookupExhaustedError(
            f"ProverkaCheka did not finish after {self._max_attempts} attempts"
        )

    async def _send(
        self, body: bytes, headers: Mapping[str, str]
    ) -> HttpResponse:
        try:
            response = await self._transport(
                self._endpoint,
                body,
                headers,
                self._timeout_seconds,
                self._max_response_bytes,
            )
        except asyncio.CancelledError:
            raise
        except ReceiptLookupError:
            raise
        except Exception as exc:
            raise ReceiptLookupTransportError("ProverkaCheka request failed") from exc
        if not isinstance(response, HttpResponse):
            raise ReceiptLookupTransportError("transport returned an invalid response")
        if len(response.body) > self._max_response_bytes:
            raise ReceiptLookupResponseError("ProverkaCheka response exceeds size limit")
        return response


async def _urllib_transport(
    url: str,
    body: bytes,
    headers: Mapping[str, str],
    timeout_seconds: float,
    max_response_bytes: int,
) -> HttpResponse:
    """Perform one bounded request without adding a runtime dependency."""

    def send() -> HttpResponse:
        request = Request(url, data=body, headers=dict(headers), method="POST")
        try:
            with urlopen(request, timeout=timeout_seconds) as response:  # noqa: S310
                status = response.status
                response_body = response.read(max_response_bytes + 1)
        except HTTPError as exc:
            status = exc.code
            response_body = exc.read(max_response_bytes + 1)
        except (URLError, OSError) as exc:
            raise ReceiptLookupTransportError("ProverkaCheka request failed") from exc
        return HttpResponse(status=status, body=response_body)

    return await asyncio.to_thread(send)


def _encode_multipart(token: str, image_bytes: bytes) -> tuple[bytes, str]:
    boundary = f"----delim-{uuid4().hex}"
    prefix = (
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="token"\r\n\r\n'
        f"{token}\r\n"
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="qrfile"; filename="receipt.png"\r\n'
        "Content-Type: image/png\r\n\r\n"
    ).encode()
    suffix = f"\r\n--{boundary}--\r\n".encode()
    return prefix + image_bytes + suffix, f"multipart/form-data; boundary={boundary}"


def _parse_api_response(response: HttpResponse) -> Mapping[str, object]:
    if not 200 <= response.status < 300:
        raise ReceiptLookupResponseError(
            f"ProverkaCheka returned HTTP {response.status}"
        )
    try:
        payload = json.loads(response.body)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ReceiptLookupResponseError("ProverkaCheka returned invalid JSON") from exc
    if not isinstance(payload, dict):
        raise ReceiptLookupResponseError("ProverkaCheka response must be an object")
    code = payload.get("code")
    if not isinstance(code, int):
        raise ReceiptLookupResponseError("ProverkaCheka response has no integer code")
    return payload


def _parse_receipt(data: object) -> ReceiptLookupResult:
    if not isinstance(data, dict):
        raise ReceiptLookupResponseError("ProverkaCheka success response has no data")
    receipt = data.get("json")
    if not isinstance(receipt, dict):
        raise ReceiptLookupResponseError("ProverkaCheka success response has no receipt")
    raw_items = receipt.get("items")
    if not isinstance(raw_items, list) or not raw_items:
        raise ReceiptLookupResponseError("ProverkaCheka receipt has no items")
    items = tuple(_parse_item(item, index) for index, item in enumerate(raw_items))
    return ReceiptLookupResult(
        total_minor=_optional_int(receipt.get("totalSum"), "totalSum"),
        seller_name=_optional_string(receipt.get("user"), "user"),
        seller_inn=_optional_string(receipt.get("userInn"), "userInn"),
        retail_place_address=_optional_string(
            receipt.get("retailPlaceAddress"), "retailPlaceAddress"
        ),
        ticket_date=_optional_string(receipt.get("ticketDate"), "ticketDate"),
        items=items,
    )


def _parse_item(raw: object, index: int) -> ReceiptItem:
    if not isinstance(raw, dict):
        raise ReceiptLookupResponseError(f"receipt item {index} must be an object")
    name = raw.get("name")
    if not isinstance(name, str) or not name.strip():
        raise ReceiptLookupResponseError(f"receipt item {index} has no name")
    return ReceiptItem(
        name=name,
        price_minor=_required_int(raw.get("price"), f"receipt item {index} price"),
        quantity=_required_decimal(raw.get("quantity"), f"receipt item {index} quantity"),
        total_minor=_required_int(raw.get("sum"), f"receipt item {index} sum"),
    )


def _required_int(value: object, field: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise ReceiptLookupResponseError(f"{field} must be an integer")
    return value


def _optional_int(value: object, field: str) -> int | None:
    if value is None:
        return None
    return _required_int(value, field)


def _required_decimal(value: object, field: str) -> Decimal:
    if isinstance(value, bool) or not isinstance(value, (int, float, str)):
        raise ReceiptLookupResponseError(f"{field} must be numeric")
    try:
        parsed = Decimal(str(value))
    except (InvalidOperation, ValueError) as exc:
        raise ReceiptLookupResponseError(f"{field} must be numeric") from exc
    if not parsed.is_finite() or parsed <= 0:
        raise ReceiptLookupResponseError(f"{field} must be positive")
    return parsed


def _optional_string(value: object, field: str) -> str | None:
    if value is None:
        return None
    if not isinstance(value, str):
        raise ReceiptLookupResponseError(f"{field} must be a string")
    return value
