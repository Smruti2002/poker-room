import { defineConfig, devices } from '@playwright/test'

// End-to-end tests run against the real Go server serving the built web app,
// exactly as in production.
const port = 4173

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://localhost:${port}`,
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    // Most people play on phones: rerun the key flows on a phone-sized,
    // touch screen.
    { name: 'phone', use: { ...devices['Pixel 7'] }, grep: /@phone/ },
  ],
  webServer: {
    // Many tests create rooms from one address, so lift the per-IP limit.
    command: `cd .. && make build >/dev/null && PORT=${port} STATIC_DIR=web/dist ROOMS_PER_MINUTE=1000 HISTORY_DIR=$(mktemp -d) ./bin/server`,
    url: `http://localhost:${port}/healthz`,
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
  },
})
