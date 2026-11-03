import { useState } from 'react'
import type { Action } from '../protocol/types.ts'
import { Card } from './Card.tsx'

/**
 * Double/Triple Texas: give two of your cards to each board. Tap a board to
 * choose where your next card goes (by default the first with room), tap a
 * card in your hand to place it, and tap a placed card to take it back.
 * Nobody sees your split until the showdown.
 */
export function SplitPicker({
  hole,
  boards,
  onAct,
}: {
  hole: string[]
  boards: string[][]
  onAct: (a: Action) => void
}) {
  const [slots, setSlots] = useState<string[][]>(() => boards.map(() => []))
  const [target, setTarget] = useState(0)
  const placed = new Set(slots.flat())
  const inHand = hole.filter((c) => !placed.has(c))
  const complete = slots.every((s) => s.length === 2)

  const place = (card: string) =>
    setSlots((prev) => {
      const b = prev[target]?.length < 2 ? target : prev.findIndex((s) => s.length < 2)
      if (b < 0) return prev
      const next = prev.map((s, i) => (i === b ? [...s, card] : s))
      // Once a board is full, aim at the next one with room.
      if (next[b].length === 2) setTarget(Math.max(0, next.findIndex((s) => s.length < 2)))
      return next
    })
  const unplace = (card: string) => setSlots((prev) => prev.map((s) => s.filter((c) => c !== card)))

  return (
    <div className="split-picker" role="group" aria-label="Split your cards">
      <p className="show-hint">Give two cards to each board. Your split stays hidden until the showdown.</p>
      {boards.map((board, b) => (
        <div
          className={`split-row${b === target ? ' split-target' : ''}`}
          key={b}
          onClick={() => setTarget(b)}
          aria-label={`Board ${b + 1}`}
          aria-current={b === target}
        >
          <span className="split-label">Board {b + 1}</span>
          <div className="split-board">
            {board.map((c) => (
              <Card key={c} code={c} small />
            ))}
          </div>
          <div className="split-slots" aria-label={`Your cards for board ${b + 1}`}>
            {[0, 1].map((k) => {
              const card = slots[b][k]
              return card ? (
                <button
                  key={k}
                  type="button"
                  className="card-pick"
                  onClick={(e) => {
                    e.stopPropagation()
                    unplace(card)
                    setTarget(b)
                  }}
                >
                  <Card code={card} small />
                </button>
              ) : (
                <span key={k} className="card card-small card-slot" aria-label="Empty slot" />
              )
            })}
          </div>
        </div>
      ))}
      <div className="split-hand" aria-label="Cards to place">
        {inHand.map((c) => (
          <button key={c} type="button" className="card-pick" onClick={() => place(c)}>
            <Card code={c} />
          </button>
        ))}
      </div>
      <div className="split-actions">
        <button
          className="btn btn-link"
          type="button"
          onClick={() => {
            setSlots(boards.map(() => []))
            setTarget(0)
          }}
        >
          Reset
        </button>
        <button className="btn btn-primary" disabled={!complete} onClick={() => onAct({ type: 'allocate', cards: slots.flat() })}>
          {complete ? 'Lock in' : `Place ${inHand.length} more`}
        </button>
      </div>
    </div>
  )
}
