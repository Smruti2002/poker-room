import { type Browser, type Page, expect, test } from '@playwright/test'

async function player(browser: Browser, link: string | null, name: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage()
  await page.goto(link ?? '/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: link ? 'Join' : 'Create room' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

test('Peekaboo: everyone rolls, peeks once, then plays Hold\'em @phone', async ({ browser }) => {
  const ann = await player(browser, null, 'Ann')
  const bob = await player(browser, ann.url(), 'Bob')
  for (const label of ["Texas Hold'em", 'Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas', 'BB', "Discard Hold'em"]) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await expect(ann.getByRole('checkbox', { name: 'Peekaboo', exact: true })).toBeChecked()
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()

  // Both roll before any betting.
  for (const p of [ann, bob]) {
    await expect(p.locator('.table-header .status')).toContainText(/roll/i)
    await expect(p.getByRole('button', { name: /^Call|^Check/ })).toHaveCount(0)
    await p.getByRole('button', { name: '🎲 Roll the die' }).click()
  }

  // Each sees the other's roll on their seat.
  const annDie = Number((await bob.getByLabel('Seat Ann').locator('.tag', { hasText: '🎲' }).textContent())?.replace(/\D/g, ''))
  expect(annDie).toBeGreaterThanOrEqual(1)
  expect(annDie).toBeLessThanOrEqual(6)

  if (annDie < 6) {
    // Bob knows which board card Ann saw, but not the card.
    await expect(bob.getByLabel('Board').getByLabel('Seen by Ann')).toHaveCount(1)
    // Ann peeks once, closes it, and can't reopen it.
    await ann.getByRole('button', { name: '👀 Peek' }).click()
    const peek = ann.getByRole('group', { name: 'Your peek' })
    await expect(peek.locator('.card:not(.card-back)')).toHaveCount(1)
    await peek.getByRole('button', { name: "Close (can't reopen)" }).click()
    await expect(ann.getByRole('button', { name: '👀 Peek' })).toHaveCount(0)
    await expect(ann.getByText(/you've peeked at/)).toBeVisible()
  } else {
    await expect(ann.getByText('You rolled a 6: no peek this hand.')).toBeVisible()
  }

  // Then ordinary betting to the end.
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
  await expect(ann.locator('.board .card:not(.card-empty)')).toHaveCount(5)
  expect(await ann.evaluate('document.documentElement.scrollWidth > window.innerWidth')).toBe(false)
})
