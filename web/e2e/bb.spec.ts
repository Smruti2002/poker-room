import { type Browser, type Page, expect, test } from '@playwright/test'

async function player(browser: Browser, link: string | null, name: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage()
  await page.goto(link ?? '/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: link ? 'Join' : 'Create room' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

test('BB: four betting rounds with three exchanges, then the showdown @phone', async ({ browser }) => {
  const ann = await player(browser, null, 'Ann')
  const bob = await player(browser, ann.url(), 'Bob')
  for (const label of ["Texas Hold'em", 'Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas']) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await expect(ann.getByRole('checkbox', { name: 'BB', exact: true })).toBeChecked()
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()

  // Five cards each, no board.
  for (const p of [ann, bob]) await expect(p.locator('.seat-me .card:not(.card-back)')).toHaveCount(5)
  await expect(ann.locator('.board .card')).toHaveCount(0)

  // Everyone checks or calls; at each exchange Ann gives back one card and
  // Bob keeps his.
  let exchanges = 0
  await expect
    .poll(
      async () => {
        for (const [p, give] of [[ann, 1], [bob, 0]] as const) {
          if ((await p.locator('.table-header .status').textContent())?.includes('Hand over')) return true
          const picker = p.getByRole('group', { name: 'Exchange cards' })
          if (await picker.isVisible()) {
            if (give) await picker.getByRole('button').first().click()
            await picker.getByRole('button', { name: give ? 'Exchange 1 card' : 'Keep all my cards' }).click()
            if (give) exchanges++
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
  expect(exchanges).toBe(3)

  // Both see how many Ann took, and the hand ended in a showdown.
  await expect(bob.getByLabel('Seat Ann')).toContainText('took 1')
  await expect(bob.locator('.result')).toContainText(/wins? \d+/)
  await expect(ann.locator('.seat-me .card:not(.card-back)')).toHaveCount(5)
})
