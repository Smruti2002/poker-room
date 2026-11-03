import type { HandView } from '../protocol/types.ts'

/**
 * After a showdown, the cards in any winning hand (per board), so the board
 * and the hole cards can light them up. Empty otherwise.
 */
export function winningCards(hand?: HandView): Set<string> {
  const out = new Set<string>()
  if (!hand?.done) return out
  for (const w of hand.winners ?? []) for (const five of w.cards ?? []) for (const c of five) out.add(c)
  return out
}
