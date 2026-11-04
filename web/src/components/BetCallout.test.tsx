import { render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { calloutText, latestCallout } from '../callouts.ts'
import { player, state } from '../test/fixtures.ts'
import { Table } from './Table.tsx'

const raised = (over: Parameters<typeof player>[0]) => player({ inHand: true, holeCount: 2, ...over })
const flop = { number: 2, variant: 'holdem', street: 'Flop', boards: [['2c', '7d', '9h']], pot: 300, toAct: 'p1', currentBet: 250, done: false }

describe('bet and raise callouts', () => {
  it('reads as a bet, a raise, or an all-in', () => {
    expect(calloutText({ type: 'bet', by: 300, to: 300, allIn: false })).toBe('Bet 300')
    expect(calloutText({ type: 'raise', by: 500, to: 1000, allIn: false })).toBe('Raised 500 → 1000')
    expect(calloutText({ type: 'raise', by: 500, to: 1000, allIn: true })).toBe('All-in · Raised 500 → 1000')
  })

  it('the latest raise is the highest total in table units, even when a 6-card player pays double', () => {
    const s = state({
      players: [
        raised({ id: 'p1', name: 'Ann', lastBet: { type: 'bet', by: 100, to: 100, allIn: false } }),
        // 150 chips at 2× is 75 units: below Cy's 120.
        raised({ id: 'p2', name: 'Bob', mult: 2, lastBet: { type: 'raise', by: 100, to: 150, allIn: false } }),
        raised({ id: 'p3', name: 'Cy', lastBet: { type: 'raise', by: 45, to: 120, allIn: false } }),
      ],
    })
    expect(latestCallout(s)?.player.name).toBe('Cy')
  })

  it("shows the raise on the raiser's seat and announces it to everyone else", () => {
    const s = state({
      phase: 'playing',
      stage: 'hand',
      you: 'p1',
      players: [
        raised({ id: 'p1', name: 'Ann', hole: ['As', 'Kd'] }),
        raised({ id: 'p2', name: 'Bob', bet: 250, lastBet: { type: 'raise', by: 200, to: 250, allIn: false } }),
      ],
      hand: flop,
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(within(screen.getByLabelText('Seat Bob')).getByText('Raised 200 → 250')).toBeInTheDocument()
    expect(screen.getByText('Bob raised 200 (to 250)')).toBeInTheDocument()
  })

  it("doesn't announce your own raise, or anything once the hand is over", () => {
    const mine = state({
      phase: 'playing',
      stage: 'hand',
      you: 'p2',
      players: [raised({ id: 'p1', name: 'Ann' }), raised({ id: 'p2', name: 'Bob', hole: ['As', 'Kd'], lastBet: { type: 'bet', by: 50, to: 50, allIn: false } })],
      hand: flop,
    })
    const { rerender } = render(<Table state={mine} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.queryByText(/Bob bet/)).toBeNull()
    expect(within(screen.getByLabelText('Seat Bob')).getByText('Bet 50')).toBeInTheDocument()

    rerender(<Table state={{ ...mine, you: 'p1', stage: 'result', hand: { ...flop, done: true } }} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.queryByText(/Bob bet/)).toBeNull()
  })
})
