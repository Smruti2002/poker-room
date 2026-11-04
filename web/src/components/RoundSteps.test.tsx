import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { HandView } from '../protocol/types.ts'
import { roundSteps } from '../roundSteps.ts'
import { RoundSteps } from './RoundSteps.tsx'

const hand = (over: Partial<HandView>): HandView => ({
  number: 1,
  variant: 'holdem',
  street: 'Pre-flop',
  boards: [[]],
  pot: 30,
  currentBet: 20,
  done: false,
  ...over,
})
const show = (h: HandView) =>
  roundSteps(h)
    .map((s) => (s.state === 'current' ? `[${s.label}]` : s.state === 'done' ? `✓${s.label}` : s.label))
    .join(' ')

describe('round steps', () => {
  it("Hold'em and Omaha: the four betting rounds and the showdown", () => {
    expect(show(hand({ street: 'Pre-flop' }))).toBe('[Pre-flop] Flop Turn River Showdown')
    expect(show(hand({ variant: 'omaha', street: 'Turn' }))).toBe('✓Pre-flop ✓Flop [Turn] River Showdown')
    expect(show(hand({ street: 'Showdown', done: true }))).toBe('✓Pre-flop ✓Flop ✓Turn ✓River [Showdown]')
  })

  it('a hand everyone folded marks its last street as reached', () => {
    expect(show(hand({ street: 'Flop', done: true }))).toBe('✓Pre-flop ✓Flop Turn River Showdown')
  })

  it("adds each variant's own step", () => {
    expect(show(hand({ variant: 'dacoit', picking: true }))).toBe('[Picking] Pre-flop Flop Turn River Showdown')
    expect(show(hand({ variant: 'dacoit', street: 'Flop' }))).toBe('✓Picking ✓Pre-flop [Flop] Turn River Showdown')
    expect(show(hand({ variant: 'show2', showing: true }))).toBe('[Showing] Pre-flop Flop Turn River Showdown')
    expect(show(hand({ variant: 'double', street: 'River', allocating: true }))).toBe(
      '✓Pre-flop ✓Flop ✓Turn ✓River [Splitting] Showdown',
    )
    expect(show(hand({ variant: 'triple', street: 'River' }))).toBe('✓Pre-flop ✓Flop ✓Turn [River] Splitting Showdown')
  })

  it('marks the current step for screen readers and as a compact pill', () => {
    render(<RoundSteps hand={hand({ street: 'Turn' })} />)
    expect(screen.getByRole('listitem', { current: 'step' })).toHaveTextContent('Turn')
    expect(document.querySelector('.round-steps-compact')).toHaveTextContent('Turn3 of 5')
  })
})
