import { defineConfig, devices } from '@playwright/test';

const appURL = process.env.E2E_APP_URL ?? 'http://localhost:5173';

export default defineConfig({
  expect: { timeout: 10_000 },
  fullyParallel: false,
  outputDir: 'test-results',
  projects: [
    {
      name: 'desktop-light',
      use: { ...devices['Desktop Chrome'], colorScheme: 'light' },
    },
    {
      name: 'max-webview-light',
      use: {
        ...devices['Desktop Chrome'],
        colorScheme: 'light',
        hasTouch: true,
        isMobile: true,
        viewport: { height: 740, width: 360 },
      },
    },
    {
      name: 'max-webview-dark',
      use: {
        ...devices['Desktop Chrome'],
        colorScheme: 'dark',
        hasTouch: true,
        isMobile: true,
        viewport: { height: 740, width: 360 },
      },
    },
  ],
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
  retries: process.env.CI ? 1 : 0,
  testDir: './e2e',
  timeout: 180_000,
  use: {
    actionTimeout: 15_000,
    baseURL: appURL,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'npm run dev -- --host 0.0.0.0 --port 5173',
    env: {
      VITE_GATEWAY_URL: process.env.E2E_GATEWAY_URL ?? 'http://localhost:8080',
    },
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
    url: appURL,
  },
  workers: 1,
});
