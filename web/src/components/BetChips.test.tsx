import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Legal } from '../protocol/types.ts'
import { ActionBar } from './ActionBar.tsx'

// Facing a bet of 20 (the big blind): a raise must go to at least 40.
const facing20: Legal = { fold: true, check: false, call: true, callUnits: 20, bet: false, raise: true, minTo: 40, maxTo: 1000 }
const chip = (name: string) => screen.getByRole('button', { name })

describe('bet chips', () => {
  it('raise by the chip amount on top of the bet faced', () => {
    const onAct = vi.fn()
    render(<ActionBar legal={facing20} bet={0} mult={1} stack={1000} onAct={onAct} />)
    fireEvent.click(chip('Chip: raise by 50'))
    expect(chip('Chip: raise by 50')).toHaveClass('bet-chip-on')
    fireEvent.click(screen.getByRole('button', { name: 'Raise to 70' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'raise', to: 70 })
  })

  it('bet the chip amount when nobody has bet', () => {
    const onAct = vi.fn()
    const open: Legal = { fold: false, check: true, call: false, callUnits: 0, bet: true, raise: false, minTo: 20, maxTo: 1000 }
    render(<ActionBar legal={open} bet={0} mult={1} stack={1000} onAct={onAct} />)
    fireEvent.click(chip('Chip: bet 250'))
    fireEvent.click(screen.getByRole('button', { name: 'Bet 250' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'bet', to: 250 })
  })

  it('a chip below the minimum raise is unavailable', () => {
    // Facing a raise to 60: the minimum re-raise is to 120.
    const legal: Legal = { ...facing20, callUnits: 60, minTo: 120 }
    render(<ActionBar legal={legal} bet={0} mult={1} stack={1000} onAct={vi.fn()} />)
    expect(chip('Chip: raise by 50')).toBeDisabled()
    expect(chip('Chip: raise by 100')).toBeEnabled()
  })

  it('a chip over your stack means all-in', () => {
    const legal: Legal = { ...facing20, maxTo: 300 }
    render(<ActionBar legal={legal} bet={0} mult={1} stack={300} onAct={vi.fn()} />)
    fireEvent.click(chip('Chip: raise by 500: all-in'))
    expect(screen.getByRole('button', { name: 'Raise to 300' })).toBeInTheDocument()
    expect(chip('Chip: all-in')).toHaveClass('bet-chip-on')
  })

  it('in pot-limit, chips over the limit are unavailable and the gold chip is the maximum', () => {
    const legal: Legal = { ...facing20, maxTo: 200 }
    render(<ActionBar legal={legal} bet={0} mult={1} stack={1000} onAct={vi.fn()} />)
    expect(chip('Chip: raise by 150')).toBeEnabled() // to 170
    expect(chip('Chip: raise by 250')).toBeDisabled() // to 270
    fireEvent.click(chip('Chip: pot-limit maximum'))
    expect(screen.getByRole('button', { name: 'Raise to 200' })).toBeInTheDocument()
  })

  it('a 6-card Dacoit player pays double for the same chip', () => {
    render(<ActionBar legal={facing20} bet={0} mult={2} stack={2000} onAct={vi.fn()} />)
    fireEvent.click(chip('Chip: raise by 100'))
    // To 120 units: 240 chips.
    expect(screen.getByRole('button', { name: 'Raise to 240' })).toBeInTheDocument()
  })

  it('no Min, ½ pot or Pot buttons any more', () => {
    render(<ActionBar legal={facing20} bet={0} mult={1} stack={1000} onAct={vi.fn()} />)
    for (const name of ['Min', '½ pot', 'Pot']) expect(screen.queryByRole('button', { name })).toBeNull()
  })
})
