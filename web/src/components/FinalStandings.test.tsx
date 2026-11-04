import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { player, state } from '../test/fixtures.ts'
import { Table } from './Table.tsx'

const ended = (you: string) =>
  state({
    phase: 'ended',
    you,
    players: [
      player({ id: 'p1', name: 'Ann', stack: 3500 }),
      player({ id: 'p2', name: 'Bob', stack: 2500, loans: 1, debt: 3000 }),
      player({ id: 'p3', name: 'Cy', stack: 2000 }),
      player({ id: 'p4', name: 'Dee', stack: 0, kicked: true }),
    ],
    // Sorted by the server, biggest winner first.
    leaderboard: [
      { id: 'p1', name: 'Ann', profit: 1500 },
      { id: 'p3', name: 'Cy', profit: 0 },
      { id: 'p4', name: 'Dee', profit: -2000, kicked: true },
      { id: 'p2', name: 'Bob', profit: -2500 },
    ],
  })
const render_ = (you: string, send = vi.fn()) => {
  render(<Table state={ended(you)} clockOffset={0} onAct={vi.fn()} send={send} />)
  return send
}

describe('final standings', () => {
  it('replaces the table: no seats, the standings in the middle', () => {
    render_('p2')
    expect(screen.getByRole('heading', { name: 'Final standings' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Seat Ann')).toBeNull()
    expect(screen.queryByRole('heading', { name: 'Leaderboard' })).toBeNull()
  })

  it('ranks by profit with the details behind each number', () => {
    render_('p2')
    const rows = screen.getAllByRole('listitem')
    expect(rows.map((r) => r.getAttribute('aria-label'))).toEqual(['1. Ann', '2. Cy', '3. Dee', '4. Bob'])
    expect(rows[0]).toHaveTextContent('🏆')
    expect(rows[0]).toHaveTextContent('+1,500')
    expect(rows[1]).toHaveTextContent('even')
    expect(within(rows[2]).getByText('removed')).toBeInTheDocument()
    expect(rows[3]).toHaveTextContent('−2,500')
    expect(rows[3]).toHaveTextContent('2,500 chips · 1 loan · 3,000 debt')
  })

  it('marks your own row', () => {
    render_('p3')
    expect(screen.getByRole('listitem', { name: '2. Cy' })).toHaveClass('final-me')
    expect(within(screen.getByRole('listitem', { name: '2. Cy' })).getByText('you')).toBeInTheDocument()
  })

  it('players on the same profit share a place, and nobody gets the trophy for breaking even', () => {
    const s = ended('p1')
    s.leaderboard = [
      { id: 'p1', name: 'Ann', profit: 0 },
      { id: 'p2', name: 'Bob', profit: 0 },
      { id: 'p3', name: 'Cy', profit: -100 },
    ]
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getAllByRole('listitem').map((r) => r.getAttribute('aria-label'))).toEqual(['1. Ann', '1. Bob', '3. Cy'])
    expect(screen.queryByText('🏆')).toBeNull()
  })

  it('the host can start a new game from here; others are told they can', () => {
    const send = render_('p1')
    fireEvent.click(screen.getByRole('button', { name: 'New game with same players' }))
    expect(send).toHaveBeenCalledWith({ type: 'rematch' })
  })
})
