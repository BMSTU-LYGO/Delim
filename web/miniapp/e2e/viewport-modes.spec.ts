import { execFileSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test, type Page } from '@playwright/test';

interface Actors {
  member: { id: number; name: string; token: string };
  owner: { id: number; name: string; token: string };
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

const useSession = async (page: Page, token: string) => {
  await page.addInitScript(
    ([key, value]) => {
      if (!window.sessionStorage.getItem(key)) window.sessionStorage.setItem(key, value);
    },
    [sessionKey, token] as const,
  );
};

test.describe('проверка MAX-вьюпортов', () => {
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
      data: { name: `E2E вьюпорт ${Date.now()}` },
      headers: { Authorization: `Bearer ${actors.owner.token}` },
    });
    expect(response.ok()).toBeTruthy();
    groupId = (await response.json()).id as number;
    return groupId;
  };

  test('нет горизонтального переполнения на списке групп', async ({ page }) => {
    await useSession(page, actors.owner.token);
    await page.goto('/');
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(1);
  });

  test('sticky-действия видны при низкой высоте (виртуальная клавиатура)', async ({ page }) => {
    await useSession(page, actors.owner.token);
    const group = await ensureGroup(page);
    await page.goto(`/groups/${group}/expense/new`);
    // Emulate an on-screen keyboard shrinking the visual viewport.
    await page.setViewportSize({ width: page.viewportSize()?.width ?? 360, height: 360 });
    const bar = page.locator('.sticky-action-bar');
    await expect(bar).toBeVisible();
    const box = await bar.boundingBox();
    const viewport = page.viewportSize();
    expect(box).not.toBeNull();
    expect((box?.y ?? 0) + (box?.height ?? 0)).toBeLessThanOrEqual((viewport?.height ?? 0) + 1);
    await expect(bar.getByRole('button', { name: 'Сохранить расход' })).toBeVisible();
  });

  test('адаптер BackButton: в MAX внутренняя кнопка скрыта, вне MAX видна', async ({
    page,
  }, testInfo) => {
    await useSession(page, actors.owner.token);
    const group = await ensureGroup(page);
    await page.goto(`/groups/${group}/members`);
    const inAppBack = page.getByRole('button', { name: 'Назад' });
    if (testInfo.project.name.startsWith('max-webview')) {
      // The MAX bridge owns the back affordance; the fallback must not render
      // a duplicate in-app button and must not crash the page.
      await expect(inAppBack).toHaveCount(0);
      await expect(page.getByRole('heading', { name: 'Участники' })).toBeVisible();
    } else {
      await expect(inAppBack).toBeVisible();
    }
  });
});
