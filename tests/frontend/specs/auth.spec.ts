import { test, expect, Page } from '@playwright/test';

// Seed users exist in the user-service DB (store/seed.go) — already activated.
const RIDER = { email: 'rider@drova.local', pass: 'Test1234!' };
const DRIVER = { email: 'driver@drova.local', pass: 'Test1234!' };

async function login(page: Page, email: string, pass: string) {
  await page.goto('/');
  await expect(page.locator('#authscreen')).toBeVisible();
  // Default tab is Sign In.
  await page.locator('#loginEmail').fill(email);
  await page.locator('#loginPassword').fill(pass);
  await page.locator('#loginform button', { hasText: 'Sign In' }).click();
}

test('fresh load shows the auth screen (no auto-login without session)', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('#authscreen')).toBeVisible();
});

test('rider can log in and reach the app', async ({ page }) => {
  await login(page, RIDER.email, RIDER.pass);
  // Rider lands on the map screen; auth screen must be gone.
  await expect(page.locator('#authscreen')).toBeHidden({ timeout: 15_000 });
});

test('driver login lands on the driver screen', async ({ page }) => {
  await login(page, DRIVER.email, DRIVER.pass);
  await expect(page.locator('#driverscreen')).toBeVisible({ timeout: 15_000 });
});

test('wrong password shows an error and stays on auth screen', async ({ page }) => {
  await login(page, RIDER.email, 'WrongPassword1!');
  await expect(page.locator('#autherr')).toBeVisible({ timeout: 10_000 });
  await expect(page.locator('#authscreen')).toBeVisible();
});

// THE regression test: logout must kill the session server-side.
// Before the fix, logout() only cleared client state — the HttpOnly refresh
// cookie survived, so a reload silently auto-logged-in again.
test('logout kills the session — reload does NOT auto-login', async ({ page }) => {
  await login(page, DRIVER.email, DRIVER.pass);
  await expect(page.locator('#driverscreen')).toBeVisible({ timeout: 15_000 });

  await page.locator('button', { hasText: 'Sign Out' }).first().click();
  await expect(page.locator('#authscreen')).toBeVisible({ timeout: 10_000 });

  // The real proof: a full reload must NOT restore the session.
  await page.reload();
  await expect(page.locator('#authscreen')).toBeVisible({ timeout: 10_000 });
  await expect(page.locator('#driverscreen')).toBeHidden();
});

// Token is genuinely revoked in the Redis blacklist after logout.
test('access token is rejected after logout', async ({ page, request }) => {
  // Capture the access token from the login response — robust against how the
  // app scopes its in-memory token variable.
  let token = '';
  page.on('response', async (res) => {
    if (res.url().endsWith('/v1/auth/token') && res.ok()) {
      try {
        const body = await res.json();
        token = body?.data?.token || body?.token || '';
      } catch { /* ignore non-JSON */ }
    }
  });

  await login(page, RIDER.email, RIDER.pass);
  await expect(page.locator('#authscreen')).toBeHidden({ timeout: 15_000 });
  expect(token, 'login response should carry an access token').toBeTruthy();

  // /v1/users/me works with the live token.
  const ok = await request.get('/v1/users/me', {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(ok.status()).toBe(200);

  // Log out through the UI.
  await page.locator('button', { hasText: 'Sign Out' }).first().click();
  await expect(page.locator('#authscreen')).toBeVisible({ timeout: 10_000 });

  // Same token must now be rejected (Redis blacklist) — poll to allow the
  // logout request to land server-side.
  await expect.poll(async () => {
    const res = await request.get('/v1/users/me', {
      headers: { Authorization: `Bearer ${token}` },
    });
    return res.status();
  }, { timeout: 10_000 }).toBe(401);
});
