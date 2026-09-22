import assert from 'node:assert/strict';
import test from 'node:test';

import { maxBridge } from '../src/platform/maxBridge.ts';

const installWebApp = (openCodeReader?: (fileSelect?: boolean) => Promise<string>) => {
  globalThis.window = {
    matchMedia: () => ({ matches: false }),
    WebApp: {
      initData: 'mock-init-data',
      initDataUnsafe: {},
      openCodeReader,
    },
  } as Window & typeof globalThis;
};

test('QR reader returns a decoded value from the MAX contract', async () => {
  let fileSelect: boolean | undefined;
  installWebApp(async (value) => {
    fileSelect = value;
    return '  t=20260921T1538&s=471.95&fn=7380440902376626&i=23261&fp=531766102&n=1  ';
  });

  assert.deepEqual(await maxBridge.scanQRCode(false), {
    status: 'success',
    value: 't=20260921T1538&s=471.95&fn=7380440902376626&i=23261&fp=531766102&n=1',
  });
  assert.equal(fileSelect, false);
});

test('QR reader reports a cancelled scan without an error', async () => {
  installWebApp(async () => '');
  assert.deepEqual(await maxBridge.scanQRCode(), { status: 'cancelled' });
});

test('QR reader treats a cancelled MAX reader as cancellation', async () => {
  installWebApp(async () => Promise.reject({ code: 'CANCELLED' }));
  assert.deepEqual(await maxBridge.scanQRCode(), { status: 'cancelled' });
});

test('QR reader reports an absent bridge method as unsupported', async () => {
  installWebApp();
  assert.deepEqual(await maxBridge.scanQRCode(), { status: 'unsupported' });
});

test('QR reader distinguishes camera permission denial', async () => {
  installWebApp(async () => Promise.reject({ name: 'NotAllowedError' }));
  assert.deepEqual(await maxBridge.scanQRCode(), { status: 'permission_denied' });
});

test('QR reader preserves a generic bridge failure as an error result', async () => {
  installWebApp(async () => Promise.reject(new Error('Bridge connection lost')));
  assert.deepEqual(await maxBridge.scanQRCode(), { status: 'error' });
});
