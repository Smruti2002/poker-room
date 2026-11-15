import { useState } from 'react'
import type { Action } from '../protocol/types.ts'
import { Card } from './Card.tsx'

/**
 * A Discard Hold'em discard: tap exactly `count` cards to throw away, then
 * confirm. Everyone chooses at once; nobody else sees which cards.
 */
export function DiscardPicker({
  hole,
  count,
  street,
  hint,
  onAct,
}: {
  hole: string[]
  count: number
  street: string
  hint?: string
  onAct: (a: Action) => void
}) {
  const [out, setOut] = useState<string[]>([])
  const toggle = (c: string) =>
    setOut((o) => (o.includes(c) ? o.filter((x) => x !== c) : o.length < count ? [...o, c] : o))
  const left = count - out.length
  return (
    <div className="draw-picker" role="group" aria-label="Discard cards">
      <p className="show-hint">
        {street}: tap {count} cards to throw away. Nobody sees which, and they're out for the rest of the hand.
      </p>
      {hint && (
        <p className="hint">
          With all {hole.length} you have: <strong>{hint}</strong>
        </p>
      )}
      <div className="show-cards">
        {hole.map((c) => (
          <button
            key={c}
            type="button"
            className={`card-pick${out.includes(c) ? ' card-discard' : ''}`}
            aria-pressed={out.includes(c)}
            onClick={() => toggle(c)}
          >
            <Card code={c} />
          </button>
        ))}
      </div>
      <button className="btn btn-primary" disabled={left > 0} onClick={() => onAct({ type: 'discard', cards: out })}>
        {left > 0 ? `Pick ${left} more` : `Throw away ${count} cards`}
      </button>
    </div>
  )
}
