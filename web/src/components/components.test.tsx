import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Legal } from '../protocol/types.ts'
import { player, settings, state } from '../test/fixtures.ts'
import { ActionBar } from './ActionBar.tsx'
import { Card } from './Card.tsx'
import { Lobby } from './Lobby.tsx'
import { Table } from './Table.tsx'

describe('Card', () => {
  it('renders rank and suit, red for hearts and diamonds', () => {
    render(<Card code="Td" />)
    const card = screen.getByLabelText('Ten of diamonds')
    expect(card).toHaveTextContent('10♦')
    expect(card).toHaveClass('card-red')
  })

  it('renders black suits and rejects bad codes', () => {
    render(
      <>
        <Card code="As" />
        <Card code="Zz" />
      </>,
    )
    expect(screen.getByLabelText('Ace of spades')).not.toHaveClass('card-red')
    expect(screen.getByText('?')).toBeInTheDocument()
  })
})

describe('ActionBar', () => {
  const facingBet: Legal = { fold: true, check: false, call: true, callUnits: 20, bet: false, raise: true, minTo: 40, maxTo: 1000 }

  it('offers fold, call and raise when facing a bet', () => {
    const onAct = vi.fn()
    render(<ActionBar legal={facingBet} bet={0} mult={1} pot={30} onAct={onAct} />)
    fireEvent.click(screen.getByRole('button', { name: 'Fold' }))
    fireEvent.click(screen.getByRole('button', { name: 'Call 20' }))
    fireEvent.click(screen.getByRole('button', { name: 'Raise to 40' }))
    expect(onAct.mock.calls).toEqual([[{ type: 'fold' }], [{ type: 'call' }], [{ type: 'raise', to: 40 }]])
    expect(screen.queryByRole('button', { name: 'Check' })).toBeNull()
  })

  it('pot preset and typed amounts are clamped to the legal range', () => {
    const onAct = vi.fn()
    render(<ActionBar legal={facingBet} bet={0} mult={1} pot={30} onAct={onAct} />)
    // Call 20 to make the pot 50, then raise by 50: to 70.
    fireEvent.click(screen.getByRole('button', { name: 'Pot' }))
    fireEvent.click(screen.getByRole('button', { name: 'Raise to 70' }))
    fireEvent.change(screen.getByLabelText('raise to'), { target: { value: '99999' } })
    fireEvent.click(screen.getByRole('button', { name: 'Raise to 1000' }))
    expect(onAct.mock.calls).toEqual([[{ type: 'raise', to: 70 }], [{ type: 'raise', to: 1000 }]])
  })

  it('offers check and bet when nobody has bet', () => {
    const onAct = vi.fn()
    const legal: Legal = { fold: false, check: true, call: false, callUnits: 0, bet: true, raise: false, minTo: 20, maxTo: 500 }
    render(<ActionBar legal={legal} bet={0} mult={1} pot={60} onAct={onAct} />)
    fireEvent.click(screen.getByRole('button', { name: 'Check' }))
    fireEvent.click(screen.getByRole('button', { name: 'Bet 20' }))
    expect(onAct.mock.calls).toEqual([[{ type: 'check' }], [{ type: 'bet', to: 20 }]])
    expect(screen.queryByRole('button', { name: 'Fold' })).toBeNull()
  })

  it('shows a single all-in button when that is the only raise, in chips for double-pay players', () => {
    const legal: Legal = { ...facingBet, minTo: 30, maxTo: 30 }
    render(<ActionBar legal={legal} bet={0} mult={2} pot={30} onAct={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'All-in 60' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Call 40' })).toBeInTheDocument()
    expect(screen.queryByRole('slider')).toBeNull()
  })
})

describe('Lobby', () => {
  it('lets the host save settings and start once two players are in', () => {
    const onSave = vi.fn()
    const onStart = vi.fn()
    render(<Lobby state={state()} onSave={onSave} onStart={onStart} />)

    const start = screen.getByRole('button', { name: 'Start game' })
    expect(start).toBeEnabled()

    fireEvent.change(screen.getByLabelText('Starting stack'), { target: { value: '5000' } })
    expect(start).toBeDisabled() // unsaved changes
    fireEvent.click(screen.getByRole('button', { name: 'Save settings' }))
    expect(onSave).toHaveBeenCalledWith({ ...settings, startingStack: 5000 })
  })

  it('needs two players to start', () => {
    render(<Lobby state={state({ players: [player({ id: 'p1', name: 'Ann' })] })} onSave={vi.fn()} onStart={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Start game' })).toBeDisabled()
  })

  it('shows read-only live settings to other players', () => {
    const { rerender } = render(<Lobby state={state({ you: 'p2' })} onSave={vi.fn()} onStart={vi.fn()} />)
    expect(screen.getByText('Waiting for the host to start…')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Start game' })).toBeNull()
    expect(screen.getByLabelText('Starting stack')).toBeDisabled()

    rerender(<Lobby state={state({ you: 'p2', settings: { ...settings, startingStack: 7000 } })} onSave={vi.fn()} onStart={vi.fn()} />)
    expect(screen.getByLabelText('Starting stack')).toHaveValue(7000)
  })
})

describe('Table', () => {
  const inHand = state({
    phase: 'playing',
    stage: 'hand',
    players: [
      player({ id: 'p1', name: 'Ann', inHand: true, hole: ['As', 'Kd'], bet: 10 }),
      player({ id: 'p2', name: 'Bob', inHand: true, bet: 20, dealer: true }),
      player({ id: 'p3', name: 'Cy', inHand: true, folded: true }),
    ],
    hand: {
      number: 3,
      variant: 'holdem',
      street: 'Pre-flop',
      boards: [[]],
      pot: 30,
      toAct: 'p1',
      currentBet: 20,
      deadline: Date.now() + 20_000,
      legal: { fold: true, check: false, call: true, callUnits: 10, bet: false, raise: true, minTo: 40, maxTo: 1990 },
      done: false,
    },
  })

  it('shows my cards, hides other live hands, and offers actions on my turn', () => {
    const onAct = vi.fn()
    render(<Table state={inHand} clockOffset={0} onAct={onAct} send={vi.fn()} />)

    const ann = screen.getByLabelText('Seat Ann')
    expect(within(ann).getByLabelText('Ace of spades')).toBeInTheDocument()
    expect(within(screen.getByLabelText('Seat Bob')).getAllByLabelText('Hidden card')).toHaveLength(2)
    expect(within(screen.getByLabelText('Seat Cy')).queryAllByLabelText('Hidden card')).toHaveLength(0)
    expect(screen.getByRole('status')).toHaveTextContent('Your turn')
    expect(screen.getByText(/Hand #3 · Texas Hold'em · Pre-flop/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Call 10' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'call' })
  })

  it('offers a loan to a broke player, with the penalty spelled out', () => {
    const send = vi.fn()
    const s = state({
      phase: 'playing',
      stage: 'waiting',
      players: [player({ id: 'p1', name: 'Ann', stack: 0, broke: true }), player({ id: 'p2', name: 'Bob' })],
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={send} />)
    expect(screen.getByText(/adds 3000 to your debt/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Take loan' }))
    expect(send).toHaveBeenCalledWith({ type: 'takeLoan' })
  })

  it('lets the dealer pick the variant', () => {
    const send = vi.fn()
    const s = state({
      phase: 'playing',
      stage: 'choosing',
      choosing: { dealerId: 'p1', options: ['holdem', 'omaha'], deadline: Date.now() + 30_000 },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={send} />)
    fireEvent.click(screen.getByRole('button', { name: 'Omaha' }))
    expect(send).toHaveBeenCalledWith({ type: 'chooseVariant', variant: 'omaha' })
  })
})
