import { readFile } from 'node:fs/promises'
import { type Browser, type Page, expect, test } from '@playwright/test'

async function player(browser: Browser, link: string | null, name: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage()
  await page.goto(link ?? '/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: link ? 'Join' : 'Create room' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

test('after the game, any player downloads every hand @phone', async ({ browser }) => {
  const ann = await player(browser, null, 'Ann')
  const bob = await player(browser, ann.url(), 'Bob')
  for (const label of ['Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas']) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()

  // The host ends the game during the first hand; whoever acts folds, so
  // the hand finishes and the game ends.
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

  // Bob, not the host, downloads it.
  const download = bob.waitForEvent('download')
  await bob.getByRole('button', { name: 'Download hand history' }).click()
  const file = await download
  expect(file.suggestedFilename()).toMatch(/^poker-room-[A-Z0-9]{6}\.json$/)
  const data = JSON.parse(await readFile((await file.path())!, 'utf8'))
  expect(data.hands).toHaveLength(1)
  const hand = data.hands[0]
  expect(hand.variant).toBe('holdem')
  expect(hand.seats.map((s: { name: string }) => s.name).sort()).toEqual(['Ann', 'Bob'])
  // Everyone's cards and every action are there.
  for (const s of hand.seats) expect(s.hole).toHaveLength(2)
  expect(hand.events.some((e: { action?: string }) => e.action === 'fold')).toBe(true)
  expect(data.finalStandings).toHaveLength(2)
})
