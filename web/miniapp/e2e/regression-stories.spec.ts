import { execFileSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test, type Page } from '@playwright/test';

interface Actor {
  id: number;
  name: string;
  token: string;
}

interface Actors {
  member: Actor;
  owner: Actor;
}

const miniappDir = path.dirname(fileURLToPath(new URL('../package.json', import.meta.url)));
const repositoryRoot = path.resolve(miniappDir, '../..');
const gatewayURL = process.env.E2E_GATEWAY_URL ?? 'http://localhost:8080';
const sessionKey = 'delim.session.token';

const provisionActors = (): Actors => {
  const output = execFileSync('go', ['run', './web/miniapp/e2e/fixture'], {
    cwd: repositoryRoot,
    encoding: 'utf8',
    env: { ...process.env, GOCACHE: path.join(tmpdir(), 'delim-e2e-go-build') },
  });
  return JSON.parse(output) as Actors;
};

const authHeaders = (actor: Actor) => ({ Authorization: `Bearer ${actor.token}` });

const useSession = async (page: Page, token: string) => {
  await page.addInitScript(
    ([key, value]) => {
      if (!window.sessionStorage.getItem(key)) window.sessionStorage.setItem(key, value);
    },
    [sessionKey, token] as const,
  );
};

// A small valid PNG for receipt upload attempts.
const pngBuffer = () =>
  Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9nDwAAAAACAAD' +
      'r/8UfAAAAAElFTkSuQmCC',
    'base64',
  );

test.describe('дополнительные регрессионные сценарии', () => {
  let actors: Actors;
  let groupId = 0;

  test.beforeAll(() => {
    actors = provisionActors();
  });

  test.beforeEach(async ({ page }, testInfo) => {
    if (!testInfo.project.name.startsWith('max-webview')) return;
    await page.addInitScript(() => {
      window.WebApp = {
        BackButton: { isVisible: false, hide() {}, offClick() {}, onClick() {}, show() {} },
        colorScheme: 'light',
        deviceName: 'Playwright MAX WebView',
        initData: 'e2e-session-is-already-provisioned',
        initDataUnsafe: {},
        platform: 'android',
        version: 'e2e',
      };
    });
  });

  const ensureGroup = async (page: Page) => {
    if (groupId) return groupId;
    const response = await page.request.post(`${gatewayURL}/api/v1/groups`, {
      data: { name: `E2E регрессия ${Date.now()}` },
      headers: authHeaders(actors.owner),
    });
    expect(response.ok()).toBeTruthy();
    groupId = (await response.json()).id as number;
    await page.request.post(`${gatewayURL}/api/v1/groups/${groupId}/members`, {
      data: { user_ids: [actors.member.id] },
      headers: authHeaders(actors.owner),
    });
    return groupId;
  };

  test('не аутентифицирован → экран входа', async ({ page }) => {
    // No session token and no MAX initData: the gate must present the auth
    // screen with a retry affordance rather than the app shell.
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Не удалось подключиться' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Повторить' })).toBeVisible();
  });

  test('запрещённая группа → сообщение о правах', async ({ page }) => {
    await useSession(page, actors.owner.token);
    await page.route(/\/api\/v1\/groups\/\d+$/, async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      await route.fulfill({
        status: 403,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'forbidden', message: 'forbidden' } }),
      });
    });
    await page.goto('/groups/999999');
    await expect(page.getByText('Недостаточно прав для этого действия.')).toBeVisible();
  });

  test('конфликт 409 при редактировании расхода', async ({ page }) => {
    await useSession(page, actors.owner.token);
    const group = await ensureGroup(page);
    const created = await page.request.post(`${gatewayURL}/api/v1/groups/${group}/expenses`, {
      data: {
        payer_user_id: actors.owner.id,
        amount_minor: 5_000,
        currency: 'RUB',
        description: 'Конфликт E2E',
        expense_date: new Date().toISOString(),
        split_type: 'equal',
        participants: [
          { user_id: actors.owner.id, value: 0 },
          { user_id: actors.member.id, value: 0 },
        ],
        items: [],
      },
      headers: authHeaders(actors.owner),
    });
    expect(created.ok()).toBeTruthy();
    const expenseId = (await created.json()).id as number;

    await page.route(`**/api/v1/expenses/${expenseId}`, async (route) => {
      if (route.request().method() !== 'PUT') return route.fallback();
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'conflict', message: 'conflict' } }),
      });
    });
    await page.goto(`/expenses/${expenseId}/edit`);
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();
    await expect(page.getByRole('heading', { name: 'Расход уже изменён' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Загрузить актуальную версию' })).toBeVisible();
  });

  test('Document недоступен при загрузке чека', async ({ page }) => {
    await useSession(page, actors.owner.token);
    const group = await ensureGroup(page);
    await page.route(new RegExp(`/api/v1/groups/${group}/receipts$`), async (route) => {
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'unavailable', message: 'document down' } }),
      });
    });
    await page.goto(`/groups/${group}`);
    await page.getByLabel('Выбрать изображение чека').setInputFiles({
      buffer: pngBuffer(),
      mimeType: 'image/png',
      name: 'doc-down.png',
    });
    await page.getByRole('button', { name: 'Распознать чек' }).click();
    await expect(
      page.getByText('Сервис временно недоступен. Повторите попытку позже.'),
    ).toBeVisible();
  });

  test('OCR failed и повтор распознавания', async ({ page }) => {
    await useSession(page, actors.owner.token);
    const group = await ensureGroup(page);
    const upload = await page.request.post(`${gatewayURL}/api/v1/groups/${group}/receipts`, {
      data: (() => {
        const form = new FormData();
        form.append('file', new Blob([pngBuffer()], { type: 'image/png' }), 'ocr-failed.png');
        return form;
      })(),
      headers: authHeaders(actors.owner),
    });
    // Requires the document service for a real receipt; skip gracefully if the
    // upload path is unavailable in this run.
    if (!upload.ok()) {
      test.skip(true, 'receipt upload unavailable in this environment');
      return;
    }
    const receiptId = (await upload.json()).receipt.id as number;
    let retried = false;
    await page.route(new RegExp(`/api/v1/receipts/${receiptId}/ocr$`), async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'failed', items: [], confidence: 0 }),
      });
    });
    await page.route(new RegExp(`/api/v1/receipts/${receiptId}/retry$`), async (route) => {
      retried = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 1, receipt_id: receiptId, type: 'ocr', status: 'pending', created_at: new Date().toISOString() }),
      });
    });
    await page.goto(`/receipts/${receiptId}`);
    await expect(page.getByRole('heading', { name: 'Не удалось распознать чек' })).toBeVisible();
    await page.getByRole('button', { name: 'Повторить распознавание' }).click();
    await expect.poll(() => retried).toBe(true);
  });

  test('архивированная группа доступна только на чтение', async ({ page }) => {
    await useSession(page, actors.owner.token);
    const group = await ensureGroup(page);
    const archive = await page.request.post(`${gatewayURL}/api/v1/groups/${group}/archive`, {
      headers: authHeaders(actors.owner),
    });
    expect(archive.ok()).toBeTruthy();
    await page.goto(`/groups/${group}`);
    await expect(page.getByText('Группа в архиве', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Добавить расход' })).toHaveCount(0);
    await expect(page.getByLabel('Выбрать изображение чека')).toHaveCount(0);
  });
});
