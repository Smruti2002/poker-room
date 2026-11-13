import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { HandView } from '../protocol/types.ts'
import { player, state } from '../test/fixtures.ts'
import { ResultBanner } from './ResultBanner.tsx'
import { Table } from './Table.tsx'

const hand = {
  number: 3,
  done: true,
  boards: [[]],
  winners: [{ playerId: 'p1', chips: 30 }],
} as unknown as HandView

const result = (me = {}) =>
  state({
    phase: 'playing',
    stage: 'result',
    players: [player({ id: 'p1', name: 'Ann', inHand: true, ...me }), player({ id: 'p2', name: 'Bob', inHand: true, folded: true })],
  })

describe('showing your cards after a hand', () => {
  it('offers Show my cards when the server allows it', () => {
    const onShowCards = vi.fn()
    render(<ResultBanner state={result({ canShowCards: true })} hand={hand} onNext={vi.fn()} onSkip={vi.fn()} onShowCards={onShowCards} />)
    fireEvent.click(screen.getByRole('button', { name: 'Show my cards' }))
    expect(onShowCards).toHaveBeenCalled()
  })

  it('no button otherwise', () => {
    render(<ResultBanner state={result()} hand={hand} onNext={vi.fn()} onSkip={vi.fn()} onShowCards={vi.fn()} />)
    expect(screen.queryByRole('button', { name: 'Show my cards' })).toBeNull()
  })

  it("a player who showed is tagged, with their cards face up on their seat", () => {
    const s = state({
      phase: 'playing',
      stage: 'result',
      you: 'p2',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, showedCards: true, hole: ['As', 'Kd'] }),
        player({ id: 'p2', name: 'Bob', inHand: true, folded: true }),
      ],
      hand,
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    const seat = screen.getByLabelText('Seat Ann')
    expect(seat).toHaveTextContent('showed')
    expect(seat.querySelectorAll('.card:not(.card-back)')).toHaveLength(2)
  })
})
