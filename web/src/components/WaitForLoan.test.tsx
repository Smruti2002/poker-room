import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { HandView } from '../protocol/types.ts'
import { player, state } from '../test/fixtures.ts'
import { ResultBanner } from './ResultBanner.tsx'

const hand = {
  number: 3,
  done: true,
  boards: [[]],
  winners: [{ playerId: 'p1', chips: 400 }],
} as unknown as HandView

const table = (over = {}) =>
  state({
    phase: 'playing',
    stage: 'result',
    players: [
      player({ id: 'p1', name: 'Ann' }),
      player({ id: 'p2', name: 'Bob', stack: 0, broke: true }),
      player({ id: 'p3', name: 'Cy' }),
    ],
    waitingOnLoan: ['p2'],
    ...over,
  })

describe('Next hand waits for broke players', () => {
  it('tells the table who it waits for and holds Next hand', () => {
    const onNext = vi.fn()
    render(<ResultBanner state={table({ you: 'p3' })} hand={hand} onNext={onNext} onSkip={vi.fn()} />)
    expect(screen.getByText(/Bob is out of chips\. Waiting for them to take a loan or sit out/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next hand' })).toBeDisabled()
    // Only the host can deal without Bob.
    expect(screen.queryByRole('button', { name: 'Deal without Bob' })).toBeNull()
  })

  it('the host can deal without them', () => {
    const onSkip = vi.fn()
    render(<ResultBanner state={table()} hand={hand} onNext={vi.fn()} onSkip={onSkip} />)
    fireEvent.click(screen.getByRole('button', { name: 'Deal without Bob' }))
    expect(onSkip).toHaveBeenCalledWith('p2')
  })

  it('the broke player is told the table waits for them', () => {
    render(<ResultBanner state={table({ you: 'p2', hostId: 'p1' })} hand={hand} onNext={vi.fn()} onSkip={vi.fn()} />)
    expect(screen.getByText(/You're out of chips: take a loan or sit out/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Deal without/ })).toBeNull()
  })

  it('Next hand works as usual when nobody is broke', () => {
    const onNext = vi.fn()
    render(<ResultBanner state={table({ waitingOnLoan: undefined })} hand={hand} onNext={onNext} onSkip={vi.fn()} />)
    expect(screen.queryByText(/out of chips/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Next hand' }))
    expect(onNext).toHaveBeenCalled()
  })
})
