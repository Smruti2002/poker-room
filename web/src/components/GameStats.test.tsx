import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { GameStats as Stats } from '../protocol/types.ts'
import { player, state } from '../test/fixtures.ts'
import { GameStats } from './GameStats.tsx'

const stats: Stats = {
  hands: 40,
  variants: [
    { variant: 'omaha', hands: 25 },
    { variant: 'holdem', hands: 15 },
  ],
  biggestPot: { chips: 4200, hand: 17, variant: 'omaha', winners: ['p2'] },
  players: [
    {
      id: 'p1',
      hands: 40,
      won: 14,
      folded: 10,
      showdowns: 12,
      showdownsWon: 9,
      biggestWin: 3000,
      net: [
        { variant: 'holdem', chips: -1200 },
        { variant: 'omaha', chips: 3400 },
      ],
    },
    {
      id: 'p2',
      hands: 40,
      won: 14,
      folded: 24,
      showdowns: 6,
      showdownsWon: 2,
      biggestWin: 4200,
      net: [{ variant: 'omaha', chips: -2200 }],
    },
  ],
}

const ended = (over = {}) =>
  state({
    phase: 'ended',
    players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob', loans: 2 })],
    stats,
    ...over,
  })

describe('game stats', () => {
  it('sums up the game and hands out awards', () => {
    render(<GameStats state={ended()} />)
    expect(screen.getByText(/40 hands · most played: Omaha \(25\)/)).toBeInTheDocument()

    const award = (title: string) => screen.getByText(title).closest('li')!
    expect(award('Biggest pot')).toHaveTextContent('4,200')
    expect(award('Biggest pot')).toHaveTextContent('Hand #17, Omaha, to Bob')
    // Ties share an award.
    expect(award('Most hands won')).toHaveTextContent('You & Bob')
    expect(award('Most hands won')).toHaveTextContent('14 each')
    expect(award('Showdown king')).toHaveTextContent('You')
    expect(award('Showdown king')).toHaveTextContent('won 75% of showdowns')
    expect(award('The rock')).toHaveTextContent('Bob')
    expect(award('The rock')).toHaveTextContent('folded 60% of hands')
    expect(award("Bank's best customer")).toHaveTextContent('Bob')
  })

  it("shows each player's line, with their best and worst game", () => {
    render(<GameStats state={ended()} />)
    const ann = screen.getByLabelText('Stats for You')
    expect(ann).toHaveTextContent('Won 14 of 40 · folded 25% · showdowns 9/12')
    expect(ann).toHaveTextContent("Best: Omaha +3,400 · Worst: Texas Hold'em −1,200")
    expect(within(screen.getByLabelText('Stats for Bob')).getByText('Omaha −2,200')).toBeInTheDocument()
  })

  it('skips awards nobody qualifies for', () => {
    const few = {
      ...stats,
      biggestPot: undefined,
      players: stats.players.map((p) => ({ ...p, showdowns: 1, showdownsWon: 1 })),
    }
    render(<GameStats state={ended({ stats: few, players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' })] })} />)
    expect(screen.queryByText('Biggest pot')).toBeNull()
    expect(screen.queryByText('Showdown king')).toBeNull()
    expect(screen.queryByText("Bank's best customer")).toBeNull()
  })

  it('shows nothing without stats', () => {
    const { container } = render(<GameStats state={ended({ stats: undefined })} />)
    expect(container).toBeEmptyDOMElement()
  })
})
