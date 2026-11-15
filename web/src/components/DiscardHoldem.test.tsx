import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { HandView } from '../protocol/types.ts'
import { roundSteps } from '../roundSteps.ts'
import { player, state } from '../test/fixtures.ts'
import { variantName } from '../variants.ts'
import { DiscardPicker } from './DiscardPicker.tsx'
import { Table } from './Table.tsx'

const hole = ['As', 'Ad', '2c', '3c', '4h', '5h']

describe("Discard Hold'em", () => {
  it('is named in the variant list', () => {
    expect(variantName('discard')).toBe("Discard Hold'em")
  })

  it('the picker takes exactly two cards, then throws them away', () => {
    const onAct = vi.fn()
    render(<DiscardPicker hole={hole} count={2} street="Flop" onAct={onAct} />)
    const go = screen.getByRole('button', { name: 'Pick 2 more' })
    expect(go).toBeDisabled()
    const cards = screen.getAllByRole('button', { pressed: false })
    fireEvent.click(cards[4])
    fireEvent.click(cards[5])
    // A third pick is ignored: two is the limit.
    fireEvent.click(cards[0])
    expect(screen.getAllByRole('button', { pressed: true })).toHaveLength(2)
    fireEvent.click(screen.getByRole('button', { name: 'Throw away 2 cards' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'discard', cards: ['4h', '5h'] })
  })

  it('tells you what you have, and who the table is waiting for', () => {
    const hand = {
      number: 1,
      variant: 'discard',
      street: 'Flop',
      discarding: true,
      boards: [['Ac', 'Kc', '9c']],
      pot: 40,
      currentBet: 0,
      done: false,
      hint: 'Three of a kind',
      legal: { discard: true, discardCount: 2, fold: false, check: false, call: false, callUnits: 0, bet: false, raise: false, minTo: 0, maxTo: 0 },
    } as unknown as HandView
    const s = state({
      phase: 'playing',
      stage: 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, hole, holeCount: 6, discarding: true }),
        player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 6, discarding: true }),
      ],
      hand,
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getByRole('status')).toHaveTextContent('Flop: choose 2 cards to throw away')
    const picker = screen.getByRole('group', { name: 'Discard cards' })
    expect(within(picker).getByText('Three of a kind')).toBeInTheDocument()
    // No betting buttons during the discard.
    expect(screen.queryByRole('button', { name: 'Check' })).toBeNull()

    // Once Ann has discarded, she waits for Bob.
    const after = state({
      ...s,
      players: [player({ id: 'p1', name: 'Ann', inHand: true, hole: hole.slice(0, 4), holeCount: 4 }), s.players[1]],
      hand: { ...hand, legal: undefined },
    })
    render(<Table state={after} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getAllByRole('status').at(-1)).toHaveTextContent('Waiting for Bob to discard')
  })

  it('shows the discards in the round steps', () => {
    const steps = (h: Partial<HandView>) =>
      roundSteps({ variant: 'discard', boards: [[]], done: false, ...h } as HandView)
        .filter((s) => s.state === 'current')
        .map((s) => s.label)
    expect(steps({ street: 'Pre-flop' })).toEqual(['Pre-flop'])
    expect(steps({ street: 'Flop', discarding: true })).toEqual(['Discard 1'])
    expect(steps({ street: 'Flop' })).toEqual(['Flop'])
    expect(steps({ street: 'Turn', discarding: true })).toEqual(['Discard 2'])
    expect(steps({ street: 'River' })).toEqual(['River'])
  })
})
