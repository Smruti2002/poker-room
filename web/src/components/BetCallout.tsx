import { calloutText, latestCallout } from '../callouts.ts'
import type { BetCallout as Callout, State } from '../protocol/types.ts'

/** The bet or raise on a seat, in place of its plain "Bet" chip. */
export function SeatCallout({ c }: { c: Callout }) {
  return (
    <span className={`bet-chip bet-callout${c.type === 'raise' ? ' bet-callout-raise' : ''}`} key={`${c.type}-${c.to}`}>
      {calloutText(c)}
    </span>
  )
}

/**
 * A one-line announcement when someone else bets or raises, so a player
 * looking at their own cards notices. It fades out on its own (a CSS
 * animation, not a game timer: nothing waits on it).
 */
export function RaiseBanner({ state }: { state: State }) {
  const latest = latestCallout(state)
  if (!latest || latest.player.id === state.you || state.hand?.done) return null
  const { player, c } = latest
  const text =
    c.type === 'raise' ? `${player.name} raised ${c.by} (to ${c.to})` : `${player.name} bet ${c.to}`
  return (
    <div className="raise-banner" role="status" key={`${state.hand?.number}-${state.hand?.street}-${player.id}-${c.to}`}>
      {c.allIn ? `${text} · all-in` : text}
    </div>
  )
}
