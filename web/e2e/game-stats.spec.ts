import { type Browser, type Page, expect, test } from '@playwright/test'

async function player(browser: Browser, link: string | null, name: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage()
  await page.goto(link ?? '/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: link ? 'Join' : 'Create room' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

test('after the game, everyone sees its stats under the standings @phone', async ({ browser }) => {
  const ann = await player(browser, null, 'Ann')
  const bob = await player(browser, ann.url(), 'Bob')
  for (const label of ['Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas', 'BB']) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()

  // End during the first hand; whoever acts folds, so it's won by a fold.
  ann.once('dialog', (d) => d.accept())
  await ann.getByRole('button', { name: 'End game' }).click()
  await expect
    .poll(async () => {
      for (const p of [ann, bob]) {
        const fold = p.getByRole('button', { name: 'Fold', exact: true })
        if (await fold.isVisible()) await fold.click()
      }
      return bob.getByRole('heading', { name: 'Final standings' }).isVisible()
    })
    .toBe(true)

  for (const page of [ann, bob]) {
    await expect(page.getByRole('heading', { name: 'Game stats' })).toBeVisible()
    await expect(page.getByText("1 hand of Texas Hold'em")).toBeVisible()
    await expect(page.getByText('Most hands won')).toBeVisible()
    await expect(page.getByLabel(/^Stats for /)).toHaveCount(2)
    // Nothing sideways on a phone.
    const overflow = await page.evaluate('document.documentElement.scrollWidth > window.innerWidth')
    expect(overflow).toBe(false)
  }
  // One player folded and one won, so their lines differ.
  await expect(bob.getByLabel('Stats for You')).toContainText(/Won [01] of 1/)
})
