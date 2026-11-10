import { type Browser, type Page, expect, test } from '@playwright/test'

async function player(browser: Browser, link: string | null, name: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage()
  await page.goto(link ?? '/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: link ? 'Join' : 'Create room' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

test('when the host ends the game, everyone sees the final standings without scrolling @phone', async ({ browser }) => {
  const ann = await player(browser, null, 'Ann')
  const link = ann.url()
  const bob = await player(browser, link, 'Bob')
  const cy = await player(browser, link, 'Cy')

  // Two variants, so the game stops at the dealer's choice and ends at once.
  for (const label of ['Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas', 'BB']) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()
  ann.once('dialog', (d) => d.accept())
  await ann.getByRole('button', { name: 'End game' }).click()

  for (const page of [ann, bob, cy]) {
    const standings = page.getByRole('heading', { name: 'Final standings' })
    await expect(standings).toBeInViewport()
    await expect(page.getByRole('listitem', { name: /^\d\. (Ann|Bob|Cy)$/ })).toHaveCount(3)
    await expect(page.getByRole('listitem', { name: /Cy$/ }).last()).toBeInViewport()
    await expect(page.locator('.seat')).toHaveCount(0)
  }
  await expect(ann.getByRole('button', { name: 'New game with same players' })).toBeInViewport()
  await expect(bob.getByText('Ann (the host) can start a new game with everyone here.')).toBeInViewport()
})
