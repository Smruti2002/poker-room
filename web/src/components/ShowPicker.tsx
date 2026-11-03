import { useState } from 'react'
import type { Action } from '../protocol/types.ts'
import { Card } from './Card.tsx'

/**
 * Show 2 Play 2's reveal step: tap the cards to show everyone, then confirm.
 * Everyone sees them before any betting.
 */
export function ShowPicker({ hole, count, onAct }: { hole: string[]; count: number; onAct: (a: Action) => void }) {
  const [picked, setPicked] = useState<string[]>([])
  const toggle = (c: string) =>
    setPicked((p) => (p.includes(c) ? p.filter((x) => x !== c) : p.length < count ? [...p, c] : p))

  return (
    <div className="show-picker" role="group" aria-label="Pick cards to show">
      <p className="show-hint">
        Pick {count} cards to show everyone. You'll play exactly one shown and one hidden card.
      </p>
      <div className="show-cards">
        {hole.map((c) => (
          <button
            key={c}
            type="button"
            className={`card-pick${picked.includes(c) ? ' card-picked' : ''}`}
            aria-pressed={picked.includes(c)}
            onClick={() => toggle(c)}
          >
            <Card code={c} />
          </button>
        ))}
      </div>
      <button
        className="btn btn-primary"
        disabled={picked.length !== count}
        onClick={() => onAct({ type: 'show', cards: picked })}
      >
        {picked.length === count ? `Show these ${count}` : `Pick ${count - picked.length} more`}
      </button>
    </div>
  )
}
