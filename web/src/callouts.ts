import type { BetCallout as Callout, PlayerView, State } from './protocol/types.ts'

/** "Raised 500 → 1000", "Bet 300", with "All-in" in front when it was. */
export function calloutText(c: Callout): string {
  const text = c.type === 'raise' ? `Raised ${c.by} → ${c.to}` : `Bet ${c.to}`
  return c.allIn ? `All-in · ${text}` : text
}

/** The latest bet or raise this street: the one with the highest total. */
export function latestCallout(state: State): { player: PlayerView; c: Callout } | null {
  let best: { player: PlayerView; c: Callout } | null = null
  for (const p of state.players) {
    const c = p.lastBet
    // Compare in table units: a 6-card Dacoit player's chips are doubled.
    if (c && (!best || c.to / p.mult > best.c.to / best.player.mult)) best = { player: p, c }
  }
  return best
}
