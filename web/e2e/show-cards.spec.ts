import { type Browser, type Page, expect, test } from '@playwright/test'

async function player(browser: Browser, link: string | null, name: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage()
  await page.goto(link ?? '/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: link ? 'Join' : 'Create room' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

test('after everyone folds, the winner may show their cards @phone', async ({ browser }) => {
  const ann = await player(browser, null, 'Ann')
  const bob = await player(browser, ann.url(), 'Bob')
  for (const label of ['Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas', 'BB']) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()

  // Whoever acts first folds, so the other wins without a showdown.
  for (const p of [ann, bob]) p.on('dialog', (d) => d.accept())
  let folder: Page | undefined
  await expect
    .poll(async () => {
      for (const p of [ann, bob]) {
        const fold = p.getByRole('button', { name: 'Fold', exact: true })
        if (await fold.isVisible()) {
          await fold.click()
          folder = p
          return true
        }
      }
      return false
    })
    .toBe(true)
  const winner = folder === ann ? bob : ann
  const winnerName = winner === ann ? 'Ann' : 'Bob'
  await expect(winner.getByRole('button', { name: 'Next hand', exact: true })).toBeVisible()

  // The folder can't see the winner's cards...
  const seat = folder!.getByLabel(`Seat ${winnerName}`)
  await expect(seat.locator('.card:not(.card-back)')).toHaveCount(0)
  // ...until the winner shows them.
  await winner.getByRole('button', { name: 'Show my cards' }).click()
  await expect(seat.locator('.card:not(.card-back)')).toHaveCount(2)
  await expect(seat).toContainText('showed')
  await expect(winner.getByRole('button', { name: 'Show my cards' })).toHaveCount(0)
})
