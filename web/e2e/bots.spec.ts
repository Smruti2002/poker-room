import { expect, test } from '@playwright/test'

test('play alone against bots: they act by themselves @phone', async ({ browser }) => {
  const ann = await (await browser.newContext()).newPage()
  await ann.goto('/')
  await ann.getByLabel('Your name').fill('Ann')
  await ann.getByRole('button', { name: 'Create room' }).click()
  await ann.getByRole('button', { name: '🤖 Add bot' }).click()
  await ann.getByRole('button', { name: '🤖 Add bot' }).click()
  await expect(ann.locator('.lobby-players li')).toHaveText([/^Ann/, /^Bot 1.*bot/, /^Bot 2.*bot/])
  for (const label of ['Omaha', 'Show 2 Play 2', 'Dacoit', 'Double Texas', 'Triple Texas', 'BB']) {
    const box = ann.getByRole('checkbox', { name: label, exact: true })
    if (await box.isChecked()) await box.click()
  }
  await ann.getByRole('button', { name: 'Save settings' }).click()
  await ann.getByRole('button', { name: 'Start game' }).click()

  // Ann only ever waits on herself: three hands, checking or calling.
  for (let hand = 1; hand <= 3; hand++) {
    await expect
      .poll(
        async () => {
          const status = (await ann.locator('.table-header .status').textContent()) ?? ''
          if (status.includes('Hand over')) return true
          expect(status).not.toMatch(/Bot \d to act/) // bots never keep the table waiting
          const check = ann.getByRole('button', { name: 'Check', exact: true })
          const call = ann.getByRole('button', { name: /^Call \d+$/ })
          if (await check.isVisible()) await check.click()
          else if (await call.isVisible()) await call.click()
          return false
        },
        { timeout: 20_000, intervals: [100] },
      )
      .toBe(true)
    await expect(ann.getByLabel('Seat Bot 1')).toBeVisible()
    await ann.getByRole('button', { name: 'Next hand', exact: true }).click()
    await expect(ann.getByText(`Hand #${hand + 1}`)).toBeVisible()
  }
  await expect(ann.getByText(/Hand #4/)).toBeVisible()
})
