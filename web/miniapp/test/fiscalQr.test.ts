import assert from 'node:assert/strict';
import test from 'node:test';

import { parseFiscalQr } from '../src/features/receipts/fiscalQr.ts';

const fixture = 't=20260921T1538&s=471.95&fn=7380440902376626&i=23261&fp=531766102&n=1';

test('parses a valid fiscal QR', () => {
  assert.deepEqual(parseFiscalQr(fixture), {
    date: '2026-09-21',
    time: '15:38',
    sum: 471.95,
    fn: '7380440902376626',
    fd: '23261',
    fp: '531766102',
    operation: 1,
  });
});

test('parses shuffled fiscal QR parameters', () => {
  assert.equal(parseFiscalQr('fp=531766102&n=1&i=23261&t=20260921T1538&fn=7380440902376626&s=471.95')?.fd, '23261');
});

test('rejects an invalid amount', () => {
  assert.equal(parseFiscalQr(fixture.replace('s=471.95', 's=invalid')), undefined);
});

test('rejects a QR missing a required field', () => {
  assert.equal(parseFiscalQr(fixture.replace('&fp=531766102', '')), undefined);
});

test('rejects an ordinary URL QR', () => {
  assert.equal(parseFiscalQr('https://example.com/receipt'), undefined);
});
