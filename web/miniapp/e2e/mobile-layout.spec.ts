import { execFileSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test, type Page } from '@playwright/test';

interface Actor { id: number; name: string; token: string }
interface Actors { member: Actor; owner: Actor }

const miniappDir = path.dirname(fileURLToPath(new URL('../package.json', import.meta.url)));
const repositoryRoot = path.resolve(miniappDir, '../..');
const gatewayURL = process.env.E2E_GATEWAY_URL ?? 'http://localhost:8080';
const sessionKey = 'delim.session.token';

const provisionActors = (): Actors => JSON.parse(execFileSync('go', ['run', './web/miniapp/e2e/fixture'], {
  cwd: repositoryRoot,
  encoding: 'utf8',
  env: { ...process.env, GOCACHE: path.join(tmpdir(), 'delim-e2e-go-build') },
})) as Actors;

const useSession = (page: Page, token: string) => page.addInitScript(([key, value]) => {
  sessionStorage.setItem(key, value);
}, [sessionKey, token] as const);

const mobileLayout = async (page: Page) => {
  const result = await page.evaluate(() => {
    const viewportWidth = document.documentElement.clientWidth;
    const visible = (element: Element) => {
      const style = getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return style.visibility !== 'hidden' && style.display !== 'none' && rect.width > 0 && rect.height > 0;
    };
    const controls = [...document.querySelectorAll('button, input:not([type="checkbox"]):not([type="radio"]), select, textarea')]
      .filter(visible)
      .map((element) => {
        const rect = element.getBoundingClientRect();
        return { bottom: rect.bottom, height: rect.height, left: rect.left, right: rect.right, tag: element.tagName };
      });
    const textEscapes = [...document.querySelectorAll('.dashboard-card, .form-field, .sticky-action-bar')].flatMap((card) => {
      const cardRect = card.getBoundingClientRect();
      const walker = document.createTreeWalker(card, NodeFilter.SHOW_TEXT);
      const escaped: string[] = [];
      let node: Node | null;
      while ((node = walker.nextNode())) {
        if (!node.textContent?.trim()) continue;
        const range = document.createRange();
        range.selectNodeContents(node);
        for (const rect of range.getClientRects()) {
          if (rect.width && (rect.left < cardRect.left - 1 || rect.right > cardRect.right + 1)) escaped.push(node.textContent.trim());
        }
      }
      return escaped;
    });
    const overlaps: string[] = [];
    controls.forEach((first, index) => controls.slice(index + 1).forEach((second) => {
      const horizontal = Math.min(first.right, second.right) - Math.max(first.left, second.left);
      const vertical = Math.min(first.bottom, second.bottom) - Math.max(first.bottom - first.height, second.bottom - second.height);
      if (horizontal > 4 && vertical > 4) overlaps.push(`${first.tag}/${second.tag}`);
    }));
    return { controls, overlaps, scrollWidth: document.documentElement.scrollWidth, textEscapes, viewportWidth };
  });
  expect(result.scrollWidth).toBeLessThanOrEqual(result.viewportWidth + 1);
  expect(result.textEscapes).toEqual([]);
  expect(result.overlaps).toEqual([]);
  for (const control of result.controls) {
    expect(control.left).toBeGreaterThanOrEqual(-1);
    expect(control.right).toBeLessThanOrEqual(result.viewportWidth + 1);
    expect(control.height).toBeGreaterThanOrEqual(28);
  }
};

test.describe.serial('mobile screens 360–430px', () => {
  let actors: Actors;
  let groupId = 0;

  test.beforeAll(() => { actors = provisionActors(); });
  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(!testInfo.project.name.startsWith('max-webview'), 'mobile-only assertions');
    await page.addInitScript(() => {
      window.WebApp = {
        BackButton: { hide() {}, offClick() {}, onClick() {}, show() {} },
        colorScheme: 'light', initData: '', initDataUnsafe: {}, platform: 'android',
        openMaxLink(url: string) { window.__e2eOpenedMaxLink = url; },
        shareMaxContent(payload: unknown) { window.__e2eSharePayload = payload; return Promise.resolve(); },
      };
    });
  });

  const ensureGroup = async (page: Page) => {
    if (groupId) return groupId;
    const created = await page.request.post(`${gatewayURL}/api/v1/groups`, {
      data: { name: `E2E mobile ${Date.now()}` }, headers: { Authorization: `Bearer ${actors.owner.token}` },
    });
    expect(created.ok()).toBeTruthy();
    groupId = (await created.json()).id as number;
    const addMember = await page.request.post(`${gatewayURL}/api/v1/groups/${groupId}/members`, {
      data: { user_ids: [actors.member.id] }, headers: { Authorization: `Bearer ${actors.owner.token}` },
    });
    expect(addMember.ok()).toBeTruthy();
    return groupId;
  };

  test('создание группы: поля, даты и CTA помещаются', async ({ page }) => {
    await useSession(page, actors.owner.token);
    await page.goto('/groups/new');
    await expect(page.getByLabel('Название')).toBeVisible();
    await expect(page.getByLabel('Начало')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Создать план' })).toBeVisible();
    await mobileLayout(page);
  });

  test('группа: invite/MAX share, чек и личные уведомления доступны', async ({ page }) => {
    await useSession(page, actors.owner.token);
    const group = await ensureGroup(page);
    await page.route(`**/api/v1/groups/${group}/invite`, async (route) => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ start_param: 'mobile-invite', deep_link: 'https://max.ru/delim_bot?startapp=mobile-invite', expires_at: '2030-01-01T00:00:00Z' }),
    }));
    await page.route('**/api/v1/max-subscription', async (route) => {
      if (route.request().method() === 'GET') return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ connected: false, bot_url: 'https://max.ru/delim_bot' }) });
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ connected: true, bot_url: 'https://max.ru/delim_bot' }) });
    });
    await page.goto(`/groups/${group}`);
    await page.evaluate(() => { window.WebApp!.initData = 'mobile-layout-e2e'; });
    await expect(page.getByRole('button', { name: 'Создать приглашение' })).toBeVisible();
    await page.getByRole('button', { name: 'Создать приглашение' }).click();
    await page.getByRole('button', { name: 'Отправить в MAX' }).click();
    await expect.poll(() => page.evaluate(() => window.__e2eSharePayload?.link)).toBe('https://max.ru/delim_bot?startapp=mobile-invite');
    await expect(page.getByLabel('Выбрать изображение чека')).toBeVisible();
    await page.getByRole('button', { name: 'Подключить уведомления' }).click();
    await expect.poll(() => page.evaluate(() => window.__e2eOpenedMaxLink)).toBe('https://max.ru/delim_bot');
    await mobileLayout(page);
  });

  test('участники, расход, баланс и кому вернуть без переполнения', async ({ page }) => {
    await useSession(page, actors.owner.token);
    const group = await ensureGroup(page);
    for (const [path, assertion] of [
      [`/groups/${group}/members`, () => page.getByRole('heading', { name: 'Участники' })],
      [`/groups/${group}/expense/new`, () => page.getByLabel('Описание')],
      [`/groups/${group}/balance`, () => page.getByRole('heading', { name: 'Баланс' })],
      [`/groups/${group}/settlements`, () => page.getByRole('heading', { name: /Погашения|Кому вернуть/ })],
    ] as const) {
      await page.goto(path);
      await expect(assertion()).toBeVisible();
      await mobileLayout(page);
    }
  });
});
