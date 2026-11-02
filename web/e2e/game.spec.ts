import { type Browser, type Page, expect, test } from '@playwright/test'

// Each player gets their own browser context: separate storage, like
// separate devices.
async function newPlayer(browser: Browser): Promise<Page> {
  const ctx = await browser.newContext()
  return ctx.newPage()
}

async function joinByLink(browser: Browser, link: string, name: string): Promise<Page> {
  const page = await newPlayer(browser)
  await page.goto(link)
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: 'Join' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

/** Plays passively (check, else call) until the hand is over on every page. */
async function playOutHand(pages: Page[]) {
  await expect
    .poll(
      async () => {
        for (const page of pages) {
          if ((await page.getByRole('status').textContent())?.includes('Hand over')) return true
          const check = page.getByRole('button', { name: 'Check', exact: true })
          const call = page.getByRole('button', { name: /^Call \d+$/ })
          if (await check.isVisible()) await check.click()
          else if (await call.isVisible()) await call.click()
        }
        return false
      },
      { timeout: 30_000, intervals: [100] },
    )
    .toBe(true)
}

async function totalChips(page: Page): Promise<number> {
  const stacks = await page.locator('.seat-stack').allTextContents()
  return stacks.reduce((sum, s) => sum + Number(s.replace(/\D/g, '')), 0)
}

test('three friends create a room, join by link and play a hand', async ({ browser }) => {
  const ann = await newPlayer(browser)
  await ann.goto('/')
  await ann.getByLabel('Your name').fill('Ann')
  await ann.getByRole('button', { name: 'Create room' }).click()
  await expect(ann).toHaveURL(/\/r\/[A-Z0-9]{6}$/)
  const link = await ann.locator('.share-link').textContent()
  expect(link).toMatch(/\/r\/[A-Z0-9]{6}$/)

  const bob = await joinByLink(browser, link!, 'Bob')
  const cy = await joinByLink(browser, link!, 'Cy')
  const players = [ann, bob, cy]

  await expect(ann.getByRole('heading', { name: 'Players (3/8)' })).toBeVisible()
  await expect(bob.getByText('Waiting for the host to start…')).toBeVisible()

  await ann.getByRole('button', { name: 'Start game' }).click()
  for (const page of players) {
    await expect(page.getByText(/Hand #1 · Texas Hold'em/)).toBeVisible()
    // Each player sees exactly their own two cards face up.
    await expect(page.locator('.seat-me .card:not(.card-back)')).toHaveCount(2)
    await expect(page.locator('.seat:not(.seat-me) .card:not(.card-back)')).toHaveCount(0)
  }

  await playOutHand(players)

  for (const page of players) {
    await expect(page.getByRole('status')).toContainText('Hand over')
    expect(await totalChips(page)).toBe(6000)
    await expect(page.locator('.leaderboard li')).toHaveCount(3)
  }

  // The next hand starts by itself after a short pause.
  await expect(bob.getByText(/Hand #2/)).toBeVisible({ timeout: 15_000 })
})

test('a player who refreshes keeps their seat and cards', async ({ browser }) => {
  const ann = await newPlayer(browser)
  await ann.goto('/')
  await ann.getByLabel('Your name').fill('Ann')
  await ann.getByRole('button', { name: 'Create room' }).click()
  const link = (await ann.locator('.share-link').textContent())!
  const bob = await joinByLink(browser, link, 'Bob')
  await ann.getByRole('button', { name: 'Start game' }).click()

  const myCards = bob.locator('.seat-me .card:not(.card-back)')
  await expect(myCards).toHaveCount(2)
  const before = await myCards.allTextContents()

  await bob.reload()
  await expect(bob.getByLabel('Your name')).toHaveCount(0) // no join form
  await expect(myCards).toHaveCount(2)
  expect(await myCards.allTextContents()).toEqual(before)
  await expect(bob.locator('.seat-me')).toContainText('Bob')
})

test('unknown room links say so', async ({ page }) => {
  await page.goto('/r/ZZZZZZ')
  await expect(page.getByRole('heading', { name: 'Room not found' })).toBeVisible()
})

test('a taken name is rejected on join', async ({ browser }) => {
  const ann = await newPlayer(browser)
  await ann.goto('/')
  await ann.getByLabel('Your name').fill('Ann')
  await ann.getByRole('button', { name: 'Create room' }).click()
  const link = (await ann.locator('.share-link').textContent())!

  const imposter = await newPlayer(browser)
  await imposter.goto(link)
  await imposter.getByLabel('Your name').fill('ann')
  await imposter.getByRole('button', { name: 'Join' }).click()
  await expect(imposter.getByRole('alert')).toContainText('taken')
})
