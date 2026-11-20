/**
 * service-accounts-ui.e2e.spec.cjs
 *
 * Drives the real console UI through the proxy, so the assertion runs against
 * the built bundle the browser actually loads. That is the only layer that can
 * catch layout-dependent defects: the combobox dropdown once opened (the input
 * expanded, the listbox mounted) yet painted no rows because the virtualizer was
 * seeded with a zero-width viewport. jsdom cannot see that; a real browser can.
 *
 * Requires: Go API on :3000, the web app built (vite build), and the proxy on
 * :3099 (the config starts it). Skips itself when the app is not built.
 */

const { test, expect } = require('@playwright/test');

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL || 'dev@vyzorix.local';
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'DevPassword123!';

/**
 * Sign in through the proxy's cookie endpoint.
 *
 * page.request shares the browser context's cookie jar, so the proxy session
 * cookie this sets is present for subsequent page.goto calls. Driving the login
 * form is avoided deliberately: it is a React-controlled form, and a headless
 * click can race the state update, which makes the spec flaky for reasons that
 * have nothing to do with what it is testing.
 */
async function signIn(page) {
  const response = await page.request.post('/v1/auth/login', {
    data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
  });
  expect(response.ok(), `login failed: ${response.status()} ${await response.text()}`).toBeTruthy();
}

test.describe('Service accounts UI', () => {
  test.skip(
    !process.env.E2E_BASE_URL && process.env.CI !== 'true' && !process.env.FORCE_UI_E2E,
    'Set FORCE_UI_E2E=1 (with the API up and the web app built) to run the UI specs.',
  );

  test('role picker dropdown shows its options', async ({ page }) => {
    await signIn(page);

    await page.goto('/admin/service-accounts');

    // Open the first account. If none exists the page shows the empty state, so
    // create one first through the UI to keep the test self-contained.
    const addAccount = page.getByRole('button', { name: /add service account/i }).first();
    await addAccount.click();
    const nameField = page.getByLabel(/display name/i).first();
    await nameField.fill(`e2e-${Date.now()}`);
    await page.getByRole('button', { name: /^create$/i }).click();

    // The create flow navigates to the detail page; open the token dialog.
    await page.getByRole('button', { name: /add service account token/i }).click();

    const basicRole = page.getByLabel(/basic role/i).first();
    await basicRole.click();

    // THE ASSERTION: the menu opens AND paints its options. A zero-height or
    // zero-width virtualized list renders the listbox but no visible rows, which
    // is the defect this guards.
    const listbox = page.getByRole('listbox');
    await expect(listbox).toBeVisible();

    for (const label of ['No basic role', 'Viewer', 'Operator', 'Admin']) {
      await expect(listbox.getByText(label, { exact: true })).toBeVisible();
    }

    // And the options have real layout — a painted row is not zero-height.
    const firstOption = listbox.getByText('Viewer', { exact: true });
    const box = await firstOption.boundingBox();
    expect(box).not.toBeNull();
    expect(box.height).toBeGreaterThan(10);
  });
});
