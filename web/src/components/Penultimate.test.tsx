import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { HandView } from '../protocol/types.ts'
import { player, state } from '../test/fixtures.ts'
import { variantName } from '../variants.ts'
import { ResultBanner } from './ResultBanner.tsx'

const result = (variant: string, desc?: string) => {
  const hand = {
    number: 2,
    variant,
    done: true,
    boards: [['2c', '7d', '9h', 'Jc', '3s']],
    winners: [{ playerId: 'p1', chips: 60, desc, cards: desc ? [['Kh', 'Kd', 'Jc', '9h', '7d']] : undefined }],
  } as unknown as HandView
  return render(
    <ResultBanner
      state={state({ phase: 'playing', stage: 'result', players: [player({ id: 'p1', name: 'Ann' })], hand })}
      hand={hand}
      onNext={vi.fn()}
      onSkip={vi.fn()}
      onShowCards={vi.fn()}
    />,
  )
}

describe('Penultimate Texas', () => {
  it('is named in the variant list', () => {
    expect(variantName('penultimate')).toBe('Penultimate Texas')
  })

  it('says the second-best hand wins at a showdown', () => {
    result('penultimate', 'Pair of Kings')
    expect(screen.getByText('Penultimate Texas: the second-best hand wins.')).toBeInTheDocument()
  })

  it('no note when everyone folded, or in other games', () => {
    result('penultimate')
    expect(screen.queryByText(/second-best/)).toBeNull()
  })

  it('no note in Hold\'em', () => {
    result('holdem', 'Pair of Kings')
    expect(screen.queryByText(/second-best/)).toBeNull()
  })
})
