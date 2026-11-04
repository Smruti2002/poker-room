import type { HandView } from './protocol/types.ts'

export interface RoundStep {
  label: string
  state: 'done' | 'current' | 'todo'
}

const BETTING = ['Pre-flop', 'Flop', 'Turn', 'River']

/**
 * The steps of a hand for its variant, each done, current or still to come:
 * Pre-flop, Flop, Turn, River, Showdown, plus the variants' own steps
 * (Dacoit's picking, Show 2 Play 2's showing, Double/Triple Texas' split).
 */
export function roundSteps(hand: HandView): RoundStep[] {
  const labels = [
    ...(hand.variant === 'dacoit' ? ['Picking'] : []),
    ...(hand.variant === 'show2' ? ['Showing'] : []),
    ...BETTING,
    ...(hand.variant === 'double' || hand.variant === 'triple' ? ['Splitting'] : []),
    'Showdown',
  ]
  const current = hand.picking
    ? 'Picking'
    : hand.showing
      ? 'Showing'
      : hand.allocating
        ? 'Splitting'
        : labels.includes(hand.street)
          ? hand.street
          : 'Pre-flop'
  const at = labels.indexOf(current)
  return labels.map((label, i) => ({
    label,
    // A finished hand that never reached the showdown (everyone folded)
    // still marks its last street as reached, not as in progress.
    state: i < at || (hand.done && i === at && label !== 'Showdown') ? 'done' : i === at ? 'current' : 'todo',
  }))
}
