import { type Browser, type Page, expect, test } from '@playwright/test'

async function player(browser: Browser, link: string | null, name: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage()
  await page.goto(link ?? '/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: link ? 'Join' : 'Create room' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

test('Penultimate Texas: played like Hold\'em, the second-best hand wins @phone', async ({ browser }) => {
  const ann = await player(browser, null, 'Ann')
  const bob = await player(browser, ann.url(), 'Bob')
  for (const label of ["Texas Hold'em", 'Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas', 'BB', "Discard Hold'em", 'Peekaboo']) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await expect(ann.getByRole('checkbox', { name: 'Penultimate Texas', exact: true })).toBeChecked()
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()
  await expect(ann.getByText(/Penultimate Texas/).first()).toBeVisible()

  // Check or call to the showdown.
  await expect
    .poll(
      async () => {
        for (const p of [ann, bob]) {
          if ((await p.locator('.table-header .status').textContent())?.includes('Hand over')) return true
          const check = p.getByRole('button', { name: 'Check', exact: true })
          const call = p.getByRole('button', { name: /^Call \d+$/ })
          if (await check.isVisible()) await check.click()
          else if (await call.isVisible()) await call.click()
        }
        return false
      },
      { timeout: 30_000, intervals: [100] },
    )
    .toBe(true)

  // At the showdown the result says how this game is won.
  const result = bob.locator('.result')
  await expect(result).toContainText(/wins? \d+/)
  await expect(result).toContainText('the second-best hand wins')
  expect(await bob.evaluate('document.documentElement.scrollWidth > window.innerWidth')).toBe(false)
})
