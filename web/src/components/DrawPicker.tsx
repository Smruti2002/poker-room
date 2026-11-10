import { useState } from 'react'
import type { Action } from '../protocol/types.ts'
import { Card } from './Card.tsx'

/**
 * A BB exchange: tap the cards to give back (none to all five), then
 * confirm. Everyone chooses at once; the table only learns how many.
 */
export function DrawPicker({ hole, round, onAct }: { hole: string[]; round: number; onAct: (a: Action) => void }) {
  const [out, setOut] = useState<string[]>([])
  const toggle = (c: string) => setOut((o) => (o.includes(c) ? o.filter((x) => x !== c) : [...o, c]))
  return (
    <div className="draw-picker" role="group" aria-label="Exchange cards">
      <p className="show-hint">
        Exchange {round} of 3: tap the cards to give back. You get the same number of new ones.
      </p>
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
      <button className="btn btn-primary" onClick={() => onAct({ type: 'draw', cards: out })}>
        {out.length === 0 ? 'Keep all my cards' : `Exchange ${out.length} card${out.length === 1 ? '' : 's'}`}
      </button>
    </div>
  )
}
