import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Legal } from '../protocol/types.ts'
import { ActionBar } from './ActionBar.tsx'

// Nobody has bet: checking is free, and folding is allowed too.
const free: Legal = { fold: true, check: true, call: false, callUnits: 0, bet: true, raise: false, minTo: 20, maxTo: 1000 }

describe('fold when you could check', () => {
  afterEach(() => vi.restoreAllMocks())

  it('offers Fold next to Check, and asks before folding a free hand', () => {
    const onAct = vi.fn()
    const confirm = vi.spyOn(window, 'confirm').mockReturnValueOnce(false).mockReturnValueOnce(true)
    render(<ActionBar legal={free} bet={0} mult={1} stack={1000} onAct={onAct} />)
    expect(screen.getByRole('button', { name: 'Check' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Fold' }))
    expect(confirm).toHaveBeenCalledTimes(1)
    expect(onAct).not.toHaveBeenCalled() // changed their mind

    fireEvent.click(screen.getByRole('button', { name: 'Fold' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'fold' })
  })

  it('folds straight away when facing a bet', () => {
    const onAct = vi.fn()
    const confirm = vi.spyOn(window, 'confirm')
    const facing: Legal = { ...free, check: false, call: true, callUnits: 20, bet: false, raise: true, minTo: 40 }
    render(<ActionBar legal={facing} bet={0} mult={1} stack={1000} onAct={onAct} />)
    fireEvent.click(screen.getByRole('button', { name: 'Fold' }))
    expect(confirm).not.toHaveBeenCalled()
    expect(onAct).toHaveBeenCalledWith({ type: 'fold' })
  })
})
