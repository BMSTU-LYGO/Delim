import { expect, test, type Page } from '@playwright/test';

const sessionKey = 'delim.session.token';

interface LoginReply {
  invite?: { group_id?: number; status: 'expired' | 'invalid' | 'joined' };
  token: string;
}

const installMAX = async (page: Page, initData = 'current-max-init-data') => {
  await page.addInitScript((data) => {
    window.WebApp = {
      BackButton: { hide() {}, offClick() {}, onClick() {}, show() {} },
      colorScheme: 'light',
      initData: data,
      initDataUnsafe: {},
      platform: 'android',
    };
  }, initData);
};

const mockSessionAPI = async (page: Page, reply: LoginReply, seen: string[]) => {
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path === '/api/v1/auth/max') {
      seen.push(await request.postData() ?? '');
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          expires_in: 3600,
          token: reply.token,
          user: { id: 17, max_user_id: 101 },
          ...(reply.invite ? { invite: reply.invite } : {}),
        }),
      });
      return;
    }
    if (path === '/api/v1/me') {
      seen.push(request.headers().authorization ?? '');
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ id: 17, max_user_id: 101, first_name: 'Max', last_name: '', username: 'max' }),
      });
      return;
    }
    if (path === '/api/v1/groups') {
      await route.fulfill({ contentType: 'application/json', body: JSON.stringify({ groups: [] }) });
      return;
    }
    await route.fulfill({ contentType: 'application/json', status: 404, body: JSON.stringify({ error: {} }) });
  });
};

test('первый запуск использует актуальные MAX initData', async ({ page }) => {
  const seen: string[] = [];
  await installMAX(page);
  await mockSessionAPI(page, { token: 'first-token' }, seen);

  await page.goto('/');
  await expect(page.getByRole('heading', { level: 1, name: 'Ваши планы' })).toBeVisible();
  expect(seen).toContain(JSON.stringify({ init_data: 'current-max-init-data' }));
  expect(seen).toContain('Bearer first-token');
});

test('invite нового пользователя открывает присоединённую группу', async ({ page }) => {
  const seen: string[] = [];
  await installMAX(page);
  await mockSessionAPI(page, { token: 'invite-token', invite: { status: 'joined', group_id: 42 } }, seen);

  await page.goto('/');
  await expect(page).toHaveURL(/\/groups\/42$/);
  expect(seen).toContain('Bearer invite-token');
});

test('invite обрабатывается поверх сохранённой сессии', async ({ page }) => {
  const seen: string[] = [];
  await installMAX(page);
  await page.addInitScript(([key, value]) => sessionStorage.setItem(key, value), [sessionKey, 'old-token']);
  await mockSessionAPI(page, { token: 'fresh-invite-token', invite: { status: 'joined', group_id: 42 } }, seen);

  await page.goto('/');
  await expect(page).toHaveURL(/\/groups\/42$/);
  expect(seen).toContain(JSON.stringify({ init_data: 'current-max-init-data' }));
  expect(seen).toContain('Bearer fresh-invite-token');
  expect(seen).not.toContain('Bearer old-token');
});

test('просроченное приглашение показывает понятную ошибку', async ({ page }) => {
  const seen: string[] = [];
  await installMAX(page);
  await mockSessionAPI(page, { token: 'expired-token', invite: { status: 'expired' } }, seen);

  await page.goto('/');
  await expect(page.getByText('Срок действия приглашения истёк. Попросите отправить новую ссылку.')).toBeVisible();
});
