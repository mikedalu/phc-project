import { defineConfig, devices } from '@playwright/test';

/**
 * End-to-end configuration.
 *
 * Two servers, because this application is two processes: the Go API and the
 * Vite dev server (which proxies /api to it). Both are started here so a fresh
 * clone can run `pnpm test:e2e` with no manual setup.
 *
 * One worker, deliberately. The headline test opens TWO browser contexts to
 * simulate two workstations, and running specs in parallel would have them
 * fighting over the same change log.
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  expect: { timeout: 45_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list']],

  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },

  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],

  webServer: [
    {
      // The API needs its .env. Sourcing it here mirrors `make api`.
      command: 'bash -c "set -a; . ../.env; set +a; cd ../api && go run ./cmd/api"',
      url: 'http://127.0.0.1:8080/ready',
      reuseExistingServer: true,
      timeout: 120_000,
    },
    {
      command: 'pnpm dev',
      url: 'http://localhost:5173',
      reuseExistingServer: true,
      timeout: 60_000,
    },
  ],
});
