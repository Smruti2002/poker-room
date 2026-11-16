import { type Browser, type Page, expect, test } from '@playwright/test'

async function player(browser: Browser, link: string | null, name: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage()
  await page.goto(link ?? '/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: link ? 'Join' : 'Create room' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

test("Discard Hold'em: six cards, two thrown away on the flop and turn @phone", async ({ browser }) => {
  const ann = await player(browser, null, 'Ann')
  const bob = await player(browser, ann.url(), 'Bob')
  for (const label of ["Texas Hold'em", 'Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas', 'BB', 'Peekaboo', 'Penultimate Texas']) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await expect(ann.getByRole('checkbox', { name: "Discard Hold'em", exact: true })).toBeChecked()
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()

  // Six cards each before the flop.
  for (const p of [ann, bob]) await expect(p.locator('.seat-me .card:not(.card-back)')).toHaveCount(6)

  // Everyone checks or calls; at each discard both throw away their first
  // two cards. Bob never learns which cards Ann threw away.
  const held: number[] = []
  await expect
    .poll(
      async () => {
        for (const p of [ann, bob]) {
          if ((await p.locator('.table-header .status').textContent())?.includes('Hand over')) return true
          const picker = p.getByRole('group', { name: 'Discard cards' })
          if (await picker.isVisible()) {
            if (p === ann) held.push(await picker.locator('.card-pick').count())
            await expect(picker.getByRole('button', { name: 'Pick 2 more' })).toBeDisabled()
            await picker.locator('.card-pick').nth(0).click()
            await picker.locator('.card-pick').nth(1).click()
            await picker.getByRole('button', { name: 'Throw away 2 cards' }).click()
            continue
          }
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

  // Ann discarded from six cards, then from four, and finished with two.
  expect(held).toEqual([6, 4])
  await expect(ann.locator('.board .card')).toHaveCount(5)
  await expect(bob.locator('.result')).toContainText(/wins? \d+/)
  // Showdown: both two-card hands are face up.
  await expect(bob.getByLabel('Seat Ann').locator('.card:not(.card-back)')).toHaveCount(2)
  // No sideways scrolling on a phone.
  expect(await ann.evaluate('document.documentElement.scrollWidth > window.innerWidth')).toBe(false)
})
