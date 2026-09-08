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

interface ExpensePage {
  expenses: Array<{ id: number }>;
}

const miniappDir = path.dirname(fileURLToPath(new URL('../package.json', import.meta.url)));
const repositoryRoot = path.resolve(miniappDir, '../..');
const gatewayURL = process.env.E2E_GATEWAY_URL ?? 'http://localhost:8080';
const sessionKey = 'delim.session.token';

const provisionActors = (): Actors => {
  const output = execFileSync('go', ['run', './web/miniapp/e2e/fixture'], {
    cwd: repositoryRoot,
    encoding: 'utf8',
    env: {
      ...process.env,
      GOCACHE: path.join(tmpdir(), 'delim-e2e-go-build'),
    },
  });
  return JSON.parse(output) as Actors;
};

const useSession = async (page: Page, token: string) => {
  await page.goto('/');
  await page.evaluate(
    ([key, value]) => window.sessionStorage.setItem(key, value),
    [sessionKey, token] as const,
  );
};

const switchSession = async (page: Page, token: string, target: string) => {
  await page.evaluate(
    ([key, value]) => window.sessionStorage.setItem(key, value),
    [sessionKey, token] as const,
  );
  await page.goto(target);
};

const authHeaders = (actor: Actor) => ({ Authorization: `Bearer ${actor.token}` });

const expenseIDs = async (page: Page, actor: Actor, groupId: number) => {
  const response = await page.request.get(`${gatewayURL}/api/v1/groups/${groupId}/expenses`, {
    headers: authHeaders(actor),
  });
  expect(response.ok()).toBeTruthy();
  const body = (await response.json()) as ExpensePage;
  return body.expenses.map((expense) => expense.id).sort((left, right) => left - right);
};

const confirmCurrentExpense = async (page: Page) => {
  await page.getByRole('button', { exact: true, name: 'Подтвердить' }).click();
  const dialog = page.getByRole('dialog', { name: 'Подтвердить расход?' });
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { exact: true, name: 'Подтвердить' }).click();
  await expect(page.getByText('Подтверждён', { exact: true })).toBeVisible();
};

test.describe.serial('критические пользовательские сценарии', () => {
  const groupName = `Поездка E2E ${Date.now()}`;
  let actors: Actors;
  let groupId = 0;
  let originalExpenseId = 0;

  test.beforeAll(() => {
    actors = provisionActors();
  });

  test.afterAll(async () => {
    if (!groupId) return;
    await fetch(`${gatewayURL}/api/v1/groups/${groupId}/archive`, {
      headers: authHeaders(actors.owner),
      method: 'POST',
    });
  });

  test('Story A — простой расход, подтверждение и баланс', async ({ page }) => {
    await useSession(page, actors.owner.token);
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 2, name: 'Ваши группы' })).toBeVisible();

    await page.getByRole('button', { name: 'Создать группу' }).first().click();
    await page.getByLabel('Название').fill(groupName);
    await page.getByRole('button', { name: 'Создать группу' }).click();
    await expect(page).toHaveURL(/\/groups\/\d+$/);
    groupId = Number(new URL(page.url()).pathname.split('/').at(-1));
    expect(groupId).toBeGreaterThan(0);

    const addMember = await page.request.post(
      `${gatewayURL}/api/v1/groups/${groupId}/members`,
      {
        data: { user_ids: [actors.member.id] },
        headers: authHeaders(actors.owner),
      },
    );
    expect(addMember.ok()).toBeTruthy();

    await page.getByRole('button', { name: 'Добавить расход' }).click();
    await page.getByLabel('Описание').fill('Ужин E2E');
    await page.getByLabel('Сумма').fill('100,00');
    await expect(page.getByLabel('Как разделить')).toHaveValue('equal');
    await expect(page.locator('.participant-picker input:checked')).toHaveCount(2);
    await page.getByRole('button', { name: 'Сохранить расход' }).click();
    await expect(page).toHaveURL(/\/expenses\/\d+$/);
    originalExpenseId = Number(new URL(page.url()).pathname.split('/').at(-1));
    expect(originalExpenseId).toBeGreaterThan(0);

    await confirmCurrentExpense(page);
    await page.goto(`/groups/${groupId}`);
    await page.getByRole('button', { name: 'Баланс' }).click();
    await expect(page.getByText('Тебе должны', { exact: false })).toBeVisible();
    await expect(page.getByText('Участник должен', { exact: false })).toBeVisible();
  });

  test('Story B — OCR review создаёт расход только после финального submit', async ({ page }) => {
    await useSession(page, actors.owner.token);
    const before = await expenseIDs(page, actors.owner, groupId);
    await page.goto(`/groups/${groupId}`);

    const receiptBase64 = await page.evaluate(() => {
      const canvas = document.createElement('canvas');
      canvas.width = 320;
      canvas.height = 120;
      const context = canvas.getContext('2d');
      if (!context) throw new Error('Canvas 2D is unavailable');
      context.fillStyle = '#fff';
      context.fillRect(0, 0, canvas.width, canvas.height);
      context.strokeStyle = '#000';
      context.lineWidth = 6;
      context.strokeRect(40, 30, 240, 60);
      return canvas.toDataURL('image/png').split(',')[1];
    });
    await page.getByLabel('Выбрать изображение чека').setInputFiles({
      buffer: Buffer.from(receiptBase64, 'base64'),
      mimeType: 'image/png',
      name: 'receipt-e2e.png',
    });
    await page.getByRole('button', { name: 'Распознать чек' }).click();
    await expect(page).toHaveURL(/\/receipts\/\d+$/);
    await expect(page.getByRole('heading', { level: 2, name: 'Проверьте чек' })).toBeVisible({
      timeout: 150_000,
    });
    expect(await expenseIDs(page, actors.owner, groupId)).toEqual(before);

    await page.getByLabel('Магазин или место').fill('Магазин E2E');
    await page.getByLabel('Итого').fill('60,00');
    const deleteButtons = page.getByRole('button', { name: /^Удалить позицию \d+$/ });
    while ((await deleteButtons.count()) > 0) await deleteButtons.first().click();
    await page.getByRole('button', { name: 'Добавить позицию' }).click();
    await page.getByLabel('Название позиции 1').fill('Продукты E2E');
    await page.getByLabel('Сумма позиции 1').fill('60,00');
    const item = page.locator('fieldset').filter({ hasText: 'Позиция 1' });
    const participants = item.locator('.participant-chip');
    await expect(participants).toHaveCount(2);
    await participants.nth(0).click();
    await participants.nth(1).click();
    await page.getByRole('button', { name: 'Продолжить к расходу' }).click();
    await expect(page.getByRole('heading', { level: 2, name: 'Расход из чека' })).toBeVisible();
    await page.getByRole('button', { name: 'Сохранить расход' }).click();
    await confirmCurrentExpense(page);
    expect(await expenseIDs(page, actors.owner, groupId)).toHaveLength(before.length + 1);
  });

  test('Story C — погашение подтверждает второй пользователь', async ({ page }) => {
    await useSession(page, actors.member.token);
    await page.goto(`/groups/${groupId}/balance`);
    await expect(page.getByText('Ты должен', { exact: false })).toBeVisible();
    await page.getByRole('button', { name: 'Погашения' }).click();
    await page
      .locator('.settlement-plan')
      .getByRole('button', { name: 'Отметить погашение' })
      .click();
    const settlementForm = page.locator('.settlement-form');
    await expect(settlementForm.getByLabel('Сумма')).not.toHaveValue('');
    await settlementForm.getByRole('button', { name: 'Отметить погашение' }).click();
    await expect(page.getByText('Погашение отмечено и ждёт подтверждения')).toBeVisible();

    await switchSession(page, actors.owner.token, `/groups/${groupId}/settlements`);
    await page.getByRole('button', { name: 'Подтвердить получение' }).click();
    const dialog = page.getByRole('dialog', { name: 'Деньги получены?' });
    await dialog.getByRole('button', { exact: true, name: 'Подтвердить' }).click();
    await expect(page.getByText('Подтверждено', { exact: true })).toBeVisible();

    await page.goto(`/groups/${groupId}/balance`);
    await expect(page.getByText(/Ты должен|Тебе должны/)).toHaveCount(0);
    await expect(page.getByText('Расчёты закрыты')).toHaveCount(2);
  });

  test('Story D — возврат обновляет баланс и сохраняет исходный расход', async ({ page }) => {
    await useSession(page, actors.owner.token);
    await page.goto(`/expenses/${originalExpenseId}`);
    await expect(page.getByRole('heading', { level: 2, name: 'Ужин E2E' })).toBeVisible();
    await page.getByRole('button', { name: 'Возврат или корректировка' }).click();
    await page.getByLabel('Сумма').fill('20,00');
    await page.getByRole('button', { name: 'Поровну' }).click();
    await page.getByRole('button', { name: 'Сохранить операцию' }).click();

    const history = page.locator('.adjustment-history__item').filter({ hasText: 'Возврат' });
    await expect(history).toContainText('20');
    await expect(page.getByRole('heading', { level: 2, name: 'Ужин E2E' })).toBeVisible();
    await expect(page.locator('.expense-summary')).toContainText('100');

    await page.goto(`/groups/${groupId}/balance`);
    await expect(page.getByText('Ты должен', { exact: false })).toBeVisible();
  });
});
