import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { HandView } from '../protocol/types.ts'
import { roundSteps } from '../roundSteps.ts'
import { player, state } from '../test/fixtures.ts'
import { variantName } from '../variants.ts'
import { PeekPanel, RollPicker, peekPosition } from './Peekaboo.tsx'
import { Table } from './Table.tsx'

describe('Peekaboo', () => {
  it('is named, and names board positions', () => {
    expect(variantName('peekaboo')).toBe('Peekaboo')
    expect([1, 3, 4, 5].map(peekPosition)).toEqual(['flop card 1', 'flop card 3', 'the turn', 'the river'])
  })

  it('rolls with one tap', () => {
    const onAct = vi.fn()
    render(<RollPicker onAct={onAct} />)
    fireEvent.click(screen.getByRole('button', { name: '🎲 Roll the die' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'roll' })
  })

  it('peeks once: open, close, done', () => {
    const send = vi.fn()
    const me = (over = {}) => player({ id: 'p1', name: 'Ann', inHand: true, die: 4, ...over })
    const { rerender } = render(<PeekPanel me={me({ canPeek: true })} send={send} />)
    expect(screen.getByText(/You rolled a 4: you may peek at the turn, once/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '👀 Peek' }))
    expect(send).toHaveBeenLastCalledWith({ type: 'peek' })

    rerender(<PeekPanel me={me({ peekCard: 'Qh' })} send={send} />)
    expect(screen.getByRole('group', { name: 'Your peek' })).toHaveTextContent('the turn')
    fireEvent.click(screen.getByRole('button', { name: "Close (can't reopen)" }))
    expect(send).toHaveBeenLastCalledWith({ type: 'peekDone' })

    rerender(<PeekPanel me={me({ peekUsed: true })} send={send} />)
    expect(screen.queryByRole('button', { name: '👀 Peek' })).toBeNull()
    expect(screen.getByText(/you've peeked at the turn/)).toBeInTheDocument()

    rerender(<PeekPanel me={me({ die: 6 })} send={send} />)
    expect(screen.getByText(/You rolled a 6: no peek/)).toBeInTheDocument()
  })

  it('marks who has seen each face-down board card, and the rolls on seats', () => {
    const hand = {
      number: 1,
      variant: 'peekaboo',
      street: 'Pre-flop',
      boards: [[]],
      pot: 30,
      currentBet: 20,
      done: false,
    } as unknown as HandView
    const s = state({
      phase: 'playing',
      stage: 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, die: 2, hole: ['As', 'Kd'], holeCount: 2 }),
        player({ id: 'p2', name: 'Bob', inHand: true, die: 2, holeCount: 2 }),
        player({ id: 'p3', name: 'Cy', inHand: true, die: 6, holeCount: 2 }),
      ],
      hand,
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getByLabelText('Seen by You, Bob')).toBeInTheDocument()
    expect(screen.getAllByLabelText(/^Seen by/)).toHaveLength(1)
    expect(screen.getByLabelText('Seat Cy')).toHaveTextContent('🎲 6')
  })

  it('starts with the roll in the round steps', () => {
    const current = (h: Partial<HandView>) =>
      roundSteps({ variant: 'peekaboo', boards: [[]], done: false, street: 'Pre-flop', ...h } as HandView).find(
        (s) => s.state === 'current',
      )?.label
    expect(current({ rolling: true })).toBe('Rolling')
    expect(current({})).toBe('Pre-flop')
  })
})
