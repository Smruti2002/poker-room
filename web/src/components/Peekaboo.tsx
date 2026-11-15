import type { Action, ClientMsg, PlayerView } from '../protocol/types.ts'
import { Card } from './Card.tsx'

/** Board position n (1-5) in words. */
export function peekPosition(n: number): string {
  if (n <= 3) return `flop card ${n}`
  return n === 4 ? 'the turn' : 'the river'
}

/** Peekaboo's roll: one tap, before the pre-flop betting. */
export function RollPicker({ onAct }: { onAct: (a: Action) => void }) {
  return (
    <div className="draw-picker" role="group" aria-label="Roll the die">
      <p className="show-hint">
        Roll 1–5 to peek once at that board card (1–3 the flop, 4 the turn, 5 the river). A 6 means no peek. Everyone
        sees your roll, but not the card.
      </p>
      <button className="btn btn-primary" onClick={() => onAct({ type: 'roll' })}>
        🎲 Roll the die
      </button>
    </div>
  )
}

/**
 * Your peek: after rolling 1-5 you may turn that board card over once, and
 * close it when you're done. No timer; it can't be reopened.
 */
export function PeekPanel({ me, send }: { me: PlayerView; send: (m: ClientMsg) => void }) {
  const die = me.die ?? 0
  if (die === 0) return null
  if (die === 6) return <p className="peek-note muted">🎲 You rolled a 6: no peek this hand.</p>
  const where = peekPosition(die)
  if (me.peekCard) {
    return (
      <div className="peek-panel" role="group" aria-label="Your peek">
        <span className="peek-card">
          <Card code={me.peekCard} />
        </span>
        <span>
          🎲 {die}: {where}
        </span>
        <button className="btn" onClick={() => send({ type: 'peekDone' })}>
          Close (can't reopen)
        </button>
      </div>
    )
  }
  if (me.canPeek) {
    return (
      <div className="peek-panel" role="group" aria-label="Your peek">
        <span>
          🎲 You rolled a {die}: you may peek at {where}, once.
        </span>
        <button className="btn btn-primary" onClick={() => send({ type: 'peek' })}>
          👀 Peek
        </button>
      </div>
    )
  }
  if (me.peekUsed) return <p className="peek-note muted">🎲 {die}: you've peeked at {where}.</p>
  return null
}

/** Who peeked at board position n (1-5), for the face-down board slots. */
export function peekersAt(players: PlayerView[], n: number, you: string): string[] {
  return players.filter((p) => p.inHand && p.die === n).map((p) => (p.id === you ? 'You' : p.name))
}
