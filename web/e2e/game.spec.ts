import { type Browser, type Page, expect, test } from '@playwright/test'

// Each player gets their own browser context: separate storage, like
// separate devices.
async function newPlayer(browser: Browser): Promise<Page> {
  const ctx = await browser.newContext()
  return ctx.newPage()
}

async function createRoom(browser: Browser, name: string): Promise<{ page: Page; link: string }> {
  const page = await newPlayer(browser)
  await page.goto('/')
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: 'Create room' }).click()
  await expect(page).toHaveURL(/\/r\/[A-Z0-9]{6}$/)
  const link = (await page.locator('.share-link').textContent())!
  expect(link).toMatch(/\/r\/[A-Z0-9]{6}$/)
  return { page, link }
}

async function joinByLink(browser: Browser, link: string, name: string): Promise<Page> {
  const page = await newPlayer(browser)
  await page.goto(link)
  await page.getByLabel('Your name').fill(name)
  await page.getByRole('button', { name: 'Join' }).click()
  await expect(page.getByRole('heading', { name: 'Invite friends' })).toBeVisible()
  return page
}

/** The table header's status line ("Your turn", "Hand over", ...). */
const status = (page: Page) => page.locator('.table-header .status')

/** Host leaves only the given variants enabled and saves. */
async function onlyVariants(host: Page, keep: string[]) {
  for (const label of ["Texas Hold'em", 'Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas']) {
    const box = host.getByRole('checkbox', { name: label, exact: true })
    if ((await box.isChecked()) !== keep.includes(label)) await box.click()
  }
  await host.getByRole('button', { name: 'Save settings' }).click()
  await expect(host.getByRole('button', { name: 'Start game' })).toBeEnabled()
}

/** Whoever is the dealer picks the variant. */
async function dealerPicks(pages: Page[], variant: string) {
  await expect
    .poll(
      async () => {
        for (const page of pages) {
          const button = page.locator('.choose').getByRole('button', { name: variant })
          if (await button.isVisible()) {
            await button.click()
            return true
          }
        }
        return false
      },
      { timeout: 10_000, intervals: [100] },
    )
    .toBe(true)
}

/** Plays passively (check, else call) until the hand is over on every page. */
async function playOutHand(pages: Page[]) {
  await expect
    .poll(
      async () => {
        for (const page of pages) {
          if ((await status(page).textContent())?.includes('Hand over')) return true
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
  // Every page must have the final state before anyone checks stacks.
  for (const page of pages) await expect(status(page)).toContainText('Hand over')
}

async function totalChips(page: Page): Promise<number> {
  const stacks = await page.locator('.seat-stack').allTextContents()
  return stacks.reduce((sum, s) => sum + Number(s.replace(/\D/g, '')), 0)
}

test('three friends create a room, join by link and play a hand @phone', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  const cy = await joinByLink(browser, link, 'Cy')
  const players = [ann, bob, cy]

  await expect(ann.getByRole('heading', { name: 'Players (3/8)' })).toBeVisible()
  await expect(bob.getByText('Waiting for the host to start…')).toBeVisible()

  await ann.getByRole('button', { name: 'Start game' }).click()
  await dealerPicks(players, "Texas Hold'em")
  for (const page of players) {
    await expect(page.getByText(/Hand #1 · Texas Hold'em/)).toBeVisible()
    // Each player sees exactly their own two cards face up.
    await expect(page.locator('.seat-me .card:not(.card-back)')).toHaveCount(2)
    await expect(page.locator('.seat:not(.seat-me) .card:not(.card-back)')).toHaveCount(0)
  }

  await playOutHand(players)

  for (const page of players) {
    await expect(status(page)).toContainText('Hand over')
    expect(await totalChips(page)).toBe(6000)
    await expect(page.locator('.leaderboard li')).toHaveCount(3)
    // Everyone sees who won and with what, and when the next hand starts.
    await expect(page.locator('.result')).toContainText(/wins? \d+( \(split pot\))? with /)
    // No timers: the result stays until someone taps Next hand.
    await expect(page.getByRole('button', { name: 'Next hand', exact: true })).toBeVisible()
    await expect(page.getByText(/\d+s\b/)).toHaveCount(0)
  }

  await cy.getByRole('button', { name: 'Next hand', exact: true }).click()
  await dealerPicks(players, 'Omaha')
  await expect(bob.getByText(/Hand #2 · Omaha/)).toBeVisible()
  await expect(bob.locator('.seat-me .card:not(.card-back)')).toHaveCount(4)
})

test('Show 2 Play 2: everyone reveals two cards before betting @phone', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  const cy = await joinByLink(browser, link, 'Cy')
  const players = [ann, bob, cy]
  await onlyVariants(ann, ['Show 2 Play 2'])
  await ann.getByRole('button', { name: 'Start game' }).click() // one variant: no choice needed

  // Each player in turn picks two cards to show.
  let reveals = 0
  await expect
    .poll(
      async () => {
        for (const page of players) {
          const picker = page.locator('.show-picker')
          if (await picker.isVisible()) {
            const picks = picker.locator('.card-pick')
            await picks.nth(0).click()
            await picks.nth(1).click()
            await picker.getByRole('button', { name: 'Show these 2' }).click()
            await expect(picker).toBeHidden()
            reveals++
          }
        }
        return reveals
      },
      { timeout: 20_000, intervals: [100] },
    )
    .toBe(3)

  // Betting has started, and everyone sees the others' two shown cards,
  // face up, next to two face-down ones.
  for (const page of players) {
    await expect(status(page)).not.toContainText('picking cards')
    await expect(page.locator('.seat:not(.seat-me) .card:not(.card-back)')).toHaveCount(4)
    await expect(page.locator('.seat:not(.seat-me) .card-back')).toHaveCount(4)
    await expect(page.locator('.dock .shown-card')).toHaveCount(2)
  }

  await playOutHand(players)
  for (const page of players) {
    expect(await totalChips(page)).toBe(6000)
    await expect(page.locator('.result')).toBeVisible()
  }
})

test('Dacoit: players choose 4 or 6 cards, and 6 costs double @phone', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  const cy = await joinByLink(browser, link, 'Cy')
  const players = [ann, bob, cy]
  await onlyVariants(ann, ['Dacoit'])
  await ann.getByRole('button', { name: 'Start game' }).click()

  // The two non-dealers choose in turn: the first takes 6, the second 4.
  const picks: Page[] = []
  await expect
    .poll(
      async () => {
        for (const page of players) {
          const picker = page.locator('.choose-picker')
          if (!picks.includes(page) && (await picker.isVisible())) {
            await picker.getByRole('button', { name: picks.length === 0 ? /6 cards/ : /4 cards/ }).click()
            await expect(picker).toBeHidden()
            picks.push(page)
          }
        }
        return picks.length
      },
      { timeout: 15_000, intervals: [100] },
    )
    .toBe(2)

  const [six, four] = picks
  const dealer = players.find((p) => !picks.includes(p))!
  await expect(six.locator('.dock .card')).toHaveCount(6)
  await expect(six.getByText('every bet costs you double', { exact: false })).toBeVisible()
  await expect(four.locator('.dock .card')).toHaveCount(4)
  await expect(dealer.locator('.dock .card')).toHaveCount(4)
  // Everyone sees who took 6.
  for (const page of [four, dealer]) {
    await expect(page.locator('.tag-double')).toHaveCount(1)
    await expect(page.locator('.tag-double')).toHaveText('6 cards · 2×')
  }

  await playOutHand(players)
  for (const page of players) expect(await totalChips(page)).toBe(6000)
})

test('Double Texas: everyone splits their cards, hidden until the showdown @phone', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  const cy = await joinByLink(browser, link, 'Cy')
  const players = [ann, bob, cy]
  await onlyVariants(ann, ['Double Texas'])
  await ann.getByRole('button', { name: 'Start game' }).click()

  // Check it down; the hand stops at the split instead of a showdown.
  await expect
    .poll(
      async () => {
        for (const page of players) {
          if (await page.locator('.split-picker').isVisible()) return true
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
  for (const page of players) await expect(page.locator('.split-picker')).toBeVisible()
  await expect(ann.locator('.board-row')).toHaveCount(2)

  // Ann locks in first: the others see that she has, but not how.
  const place = async (page: Page) => {
    const picker = page.locator('.split-picker')
    for (let i = 0; i < 4; i++) await picker.locator('.split-hand .card-pick').first().click()
    await picker.getByRole('button', { name: 'Lock in' }).click()
    await expect(picker).toBeHidden()
  }
  await place(ann)
  await expect(bob.getByLabel('Seat Ann').getByText('locked in')).toBeVisible()
  await expect(bob.getByLabel('Seat Ann').locator('.card:not(.card-back)')).toHaveCount(0)
  await expect(status(bob)).toContainText('Split your cards')
  await expect(status(ann)).toContainText('Waiting for Bob, Cy to split')

  await place(bob)
  await place(cy)
  for (const page of players) {
    await expect(status(page)).toContainText('Hand over')
    expect(await totalChips(page)).toBe(6000)
    // Every board shows its winner, and the banner credits boards won.
    await expect(page.locator('.board-winner')).toHaveCount(2)
    await expect(page.locator('.result')).toContainText(/Board [12]: /)
  }
})

test('chat, reactions and hand history @phone', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')

  // Chat works in the lobby; Bob sees an unread badge until he opens it.
  await ann.getByRole('button', { name: 'Chat' }).click()
  await ann.getByLabel('Message', { exact: true }).fill('good luck!')
  await ann.getByRole('button', { name: 'Send' }).click()
  await expect(ann.getByLabel('Messages')).toContainText('You good luck!')
  await expect(bob.getByRole('button', { name: 'Chat (1 unread)' })).toBeVisible()
  await bob.getByRole('button', { name: 'Chat (1 unread)' }).click()
  await expect(bob.getByLabel('Messages')).toContainText('Ann good luck!')
  await bob.getByRole('button', { name: 'Close' }).click()
  await ann.getByRole('button', { name: 'Close' }).click()
  await expect(bob.getByRole('button', { name: 'Chat', exact: true })).toBeVisible()

  // A reaction floats over the sender's seat for everyone.
  await onlyVariants(ann, ["Texas Hold'em"])
  await ann.getByRole('button', { name: 'Start game' }).click()
  await bob.getByRole('button', { name: 'Chat', exact: true }).click()
  await bob.getByRole('button', { name: 'React 😂' }).click()
  await bob.getByRole('button', { name: 'Close' }).click()
  await expect(ann.getByLabel('Bob reacts 😂')).toBeVisible()
  await expect(ann.getByLabel('Bob reacts 😂')).toBeHidden({ timeout: 8_000 })

  // After a hand, it appears in the history.
  await playOutHand([ann, bob])
  await ann.getByRole('button', { name: 'Hand history' }).click()
  await expect(ann.locator('.history summary').first()).toContainText(/#1 Texas Hold'em · .+ \+\d+/)
})

test('if the host leaves, another player can become host @phone', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  await joinByLink(browser, link, 'Cy')
  await expect(bob.getByText('Waiting for the host to start…')).toBeVisible()
  await expect(bob.getByRole('button', { name: 'Become host' })).toHaveCount(0)

  await ann.context().close() // the host walks away
  await bob.getByRole('button', { name: 'Become host' }).click()
  await expect(bob.getByRole('button', { name: 'Start game' })).toBeEnabled()
  await expect(bob.getByRole('button', { name: 'Become host' })).toHaveCount(0)
})

test('a player who refreshes keeps their seat and cards', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  await onlyVariants(ann, ["Texas Hold'em"])
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

test('the raise box can be cleared and retyped', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  await onlyVariants(ann, ["Texas Hold'em"])
  await ann.getByRole('button', { name: 'Start game' }).click()

  // Heads-up: the dealer acts first; wait for whoever gets the action bar.
  let actor: Page | undefined
  await expect
    .poll(async () => {
      for (const page of [ann, bob]) if (await page.locator('.action-bar').isVisible()) actor = page
      return actor !== undefined
    })
    .toBe(true)
  if (!actor) throw new Error('no player to act')
  const box = actor.getByLabel('raise to')
  await box.fill('')
  await expect(box).toHaveValue('')
  await box.pressSequentially('150')
  await expect(box).toHaveValue('150')
  await expect(actor.getByRole('button', { name: 'Raise to 150' })).toBeEnabled()
})

test('unknown room links say so', async ({ page }) => {
  await page.goto('/r/ZZZZZZ')
  await expect(page.getByRole('heading', { name: 'Room not found' })).toBeVisible()
})

test('a taken name is rejected on join', async ({ browser }) => {
  const { link } = await createRoom(browser, 'Ann')
  const imposter = await newPlayer(browser)
  await imposter.goto(link)
  await imposter.getByLabel('Your name').fill('ann')
  await imposter.getByRole('button', { name: 'Join' }).click()
  await expect(imposter.getByRole('alert')).toContainText('taken')
})

test('after a game, the host starts a new one with the same players and settings @phone', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  const cy = await joinByLink(browser, link, 'Cy')
  await ann.getByLabel('Starting stack').fill('5000')
  await onlyVariants(ann, ['Omaha', 'Dacoit']) // two, so the game ends at the dealer's choice
  await ann.getByRole('button', { name: 'Start game' }).click()

  ann.once('dialog', (d) => d.accept())
  await ann.getByRole('button', { name: 'End game' }).click()
  await expect(status(bob)).toHaveText('Game over')
  await expect(bob.getByText('Ann (the host) can start a new game')).toBeVisible()
  await expect(bob.getByRole('button', { name: 'New game with same players' })).toHaveCount(0)

  await cy.goto('about:blank') // Cy has stepped away
  await ann.getByRole('button', { name: 'New game with same players' }).click()

  // Ann and Bob land in the new room's lobby, already seated.
  await expect(ann).not.toHaveURL(link)
  await expect(ann).toHaveURL(/\/r\/[A-Z0-9]{6}$/)
  const next = ann.url()
  await expect(bob).toHaveURL(next)
  for (const page of [ann, bob]) {
    await expect(page.getByLabel('Your name')).toHaveCount(0) // no join form
    await expect(page.locator('.lobby-players li')).toHaveText([/^Ann/, /^Bob/, /^Cy/])
  }
  // Same settings, and Ann is still the host.
  await expect(ann.getByLabel('Starting stack')).toHaveValue('5000')
  await expect(ann.getByRole('checkbox', { name: 'Omaha', exact: true })).toBeChecked()
  await expect(ann.getByRole('checkbox', { name: 'Dacoit', exact: true })).toBeChecked()
  await expect(ann.getByRole('checkbox', { name: "Texas Hold'em", exact: true })).not.toBeChecked()
  await expect(ann.getByRole('button', { name: 'Start game' })).toBeEnabled()

  // Cy comes back to the old link and is sent on too.
  await cy.goto(link)
  await expect(cy).toHaveURL(next)
  await expect(cy.getByLabel('Your name')).toHaveCount(0)
  await ann.getByRole('button', { name: 'Start game' }).click()
  await expect(cy.locator('.seat-me')).toContainText('Cy')
  await expect(cy.locator('.seat-me .seat-stack')).toContainText('5000')
})

test('players vote someone out mid-hand @phone', async ({ browser }) => {
  const { page: ann, link } = await createRoom(browser, 'Ann')
  const bob = await joinByLink(browser, link, 'Bob')
  const cy = await joinByLink(browser, link, 'Cy')
  await onlyVariants(ann, ["Texas Hold'em"])
  await ann.getByRole('button', { name: 'Start game' }).click()
  await expect(cy.locator('.seat-me .card:not(.card-back)')).toHaveCount(2)

  ann.once('dialog', (d) => d.accept())
  await ann.getByRole('button', { name: 'Vote to remove Cy' }).click()
  // Two voters (Ann, Bob): both must agree. Cy is told, and can't vote.
  await expect(cy.getByText('The other players are voting on removing you')).toBeVisible()
  const vote = bob.getByRole('group', { name: 'Vote to remove Cy' })
  await expect(vote).toContainText('1 of 2 yes')
  await vote.getByRole('button', { name: 'Yes' }).click()

  await expect(cy.getByRole('heading', { name: 'You were removed' })).toBeVisible()
  await expect(ann.getByRole('group', { name: 'Vote to remove Cy' })).toHaveCount(0)

  // The hand finishes without waiting on Cy, and Cy is gone from the table
  // but still on the leaderboard.
  await playOutHand([ann, bob])
  await ann.getByRole('button', { name: 'Next hand', exact: true }).click()
  await expect(ann.getByLabel('Seat Cy')).toHaveCount(0)
  await expect(ann.locator('.leaderboard')).toContainText('Cyremoved')

  // Cy's link no longer gets them back in under that name.
  await cy.goto(link)
  await cy.getByLabel('Your name').fill('Cy')
  await cy.getByRole('button', { name: 'Join' }).click()
  await expect(cy.getByRole('alert')).toContainText('taken')
})
