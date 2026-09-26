import asyncio
import json
from pathlib import Path
import unittest

from delim_document.receipt_lookup import (
    HttpResponse,
    ProverkaChekaClient,
    ReceiptLookupExhaustedError,
    ReceiptLookupResponseError,
)


TESTDATA = Path(__file__).resolve().parents[1] / "testdata"


def success_response() -> HttpResponse:
    return HttpResponse(
        status=200,
        body=json.dumps(
            {
                "code": 1,
                "data": {
                    "json": {
                        "totalSum": 19900,
                        "user": "Магазин",
                        "items": [
                            {
                                "name": "Чай",
                                "price": 19900,
                                "quantity": 1,
                                "sum": 19900,
                            }
                        ],
                    }
                },
            }
        ).encode(),
    )


class ProverkaChekaClientTest(unittest.IsolatedAsyncioTestCase):
    async def test_sends_real_qr_fixture_as_multipart_qrfile(self) -> None:
        image_bytes = (TESTDATA / "proverkacheka_qr.png").read_bytes()
        captured: dict[str, object] = {}

        async def transport(url, body, headers, timeout, response_limit):
            captured.update(
                url=url,
                body=body,
                headers=headers,
                timeout=timeout,
                response_limit=response_limit,
            )
            return success_response()

        client = ProverkaChekaClient("test-token", transport=transport)
        receipt = await client.lookup(image_bytes)

        self.assertEqual(receipt.items[0].name, "Чай")
        self.assertEqual(captured["url"], "https://proverkacheka.com/api/v1/check/get")
        self.assertIn(image_bytes, captured["body"])
        self.assertIn(b'name="token"\r\n\r\ntest-token\r\n', captured["body"])
        self.assertIn(b'name="qrfile"; filename="receipt.png"', captured["body"])
        self.assertIn("multipart/form-data; boundary=", captured["headers"]["Content-Type"])

    async def test_retries_processing_codes_with_injected_sleep(self) -> None:
        responses = [
            HttpResponse(status=200, body=b'{"code": 2}'),
            HttpResponse(status=200, body=b'{"code": 4}'),
            success_response(),
        ]
        delays: list[float] = []

        async def transport(*_args):
            return responses.pop(0)

        async def sleep(delay: float) -> None:
            delays.append(delay)

        client = ProverkaChekaClient(
            "token",
            transport=transport,
            sleep=sleep,
            retry_delay_code_2_seconds=0.25,
            retry_delay_code_4_seconds=0.5,
        )
        receipt = await client.lookup(b"PNG")

        self.assertEqual(receipt.total_minor, 19900)
        self.assertEqual(delays, [0.25, 0.5])

    async def test_invalid_success_payload_is_typed_error(self) -> None:
        async def transport(*_args):
            return HttpResponse(status=200, body=b'{"code":1,"data":{"json":{"items":[]}}}')

        with self.assertRaises(ReceiptLookupResponseError):
            await ProverkaChekaClient("token", transport=transport).lookup(b"PNG")

    async def test_processing_exhaustion_is_typed_error(self) -> None:
        async def transport(*_args):
            return HttpResponse(status=200, body=b'{"code":2}')

        async def no_wait(_delay: float) -> None:
            return None

        client = ProverkaChekaClient(
            "token", transport=transport, sleep=no_wait, max_attempts=2
        )
        with self.assertRaises(ReceiptLookupExhaustedError):
            await client.lookup(b"PNG")

    async def test_cancellation_is_not_translated_to_lookup_failure(self) -> None:
        async def transport(*_args):
            raise asyncio.CancelledError

        with self.assertRaises(asyncio.CancelledError):
            await ProverkaChekaClient("token", transport=transport).lookup(b"PNG")


if __name__ == "__main__":
    unittest.main()
