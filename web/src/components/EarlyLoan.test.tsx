import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { player, state } from '../test/fixtures.ts'
import { Table } from './Table.tsx'

const table = (me: Parameters<typeof player>[0]) =>
  state({
    phase: 'playing',
    stage: 'result',
    players: [player(me), player({ id: 'p2', name: 'Bob' })],
  })

describe('early loans', () => {
  it('a player below 500 can open the offer, and borrow or back out', () => {
    const send = vi.fn()
    render(
      <Table
        state={table({ id: 'p1', name: 'Ann', stack: 300, canBorrow: true })}
        clockOffset={0}
        onAct={vi.fn()}
        send={send}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Low on chips? Take a loan' }))
    expect(screen.getByText(/You have 300 chips\. Borrow 2000/)).toBeInTheDocument()
    expect(screen.getByText(/including the 1000 penalty/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Not now' }))
    expect(send).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'Take loan' })).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Low on chips? Take a loan' }))
    fireEvent.click(screen.getByRole('button', { name: 'Take loan' }))
    expect(send).toHaveBeenCalledWith({ type: 'takeLoan' })
  })

  it('no offer with 500 or more, or during a hand', () => {
    render(<Table state={table({ id: 'p1', name: 'Ann', stack: 450 })} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.queryByRole('button', { name: /Take a loan/ })).toBeNull()
  })

  it('broke players still see the offer straight away, with no way to dismiss it', () => {
    render(
      <Table
        state={table({ id: 'p1', name: 'Ann', stack: 0, broke: true, canBorrow: true })}
        clockOffset={0}
        onAct={vi.fn()}
        send={vi.fn()}
      />,
    )
    expect(screen.getByText(/You're out of chips\. Borrow 2000/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Take loan' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Not now' })).toBeNull()
  })
})
