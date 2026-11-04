import { useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import type { SortMode } from '../cardOrder.ts'
import { Card } from './Card.tsx'

const SORT_LABEL: Record<SortMode, string> = { rank: 'Sort by rank', suit: 'Sort by suit', dealt: 'As dealt' }

/** How far a finger must move before a press becomes a drag. */
const DRAG_PX = 8

/**
 * Your cards, which you can rearrange: drag one to a new spot, or tap one
 * card then another to swap them (easier on a phone). With a card picked,
 * the arrow keys move it. The order is only for you.
 */
export function HandCards({
  cards,
  shown,
  nextSort,
  onMove,
  onSwap,
  onSort,
}: {
  cards: string[]
  shown?: string[]
  nextSort: SortMode
  onMove: (from: number, to: number) => void
  onSwap: (i: number, j: number) => void
  onSort: () => void
}) {
  const [picked, setPicked] = useState<number | null>(null)
  const [over, setOver] = useState<number | null>(null)
  const press = useRef<{ index: number; x: number; y: number; dragging: boolean } | null>(null)

  const indexAt = (x: number, y: number) => {
    for (const el of document.elementsFromPoint?.(x, y) ?? []) {
      const i = (el as HTMLElement).dataset?.cardIndex
      if (i !== undefined) return Number(i)
    }
    return null
  }

  const tap = (i: number) => {
    if (picked === null) setPicked(i)
    else if (picked === i) setPicked(null)
    else {
      onSwap(picked, i)
      setPicked(null)
    }
  }

  const onPointerDown = (i: number) => (e: PointerEvent) => {
    if (e.button !== 0) return
    press.current = { index: i, x: e.clientX, y: e.clientY, dragging: false }
    e.currentTarget.setPointerCapture?.(e.pointerId)
  }
  const onPointerMove = (e: PointerEvent) => {
    const p = press.current
    if (!p) return
    if (!p.dragging && Math.hypot(e.clientX - p.x, e.clientY - p.y) > DRAG_PX) p.dragging = true
    if (p.dragging) {
      const i = indexAt(e.clientX, e.clientY)
      setOver(i === p.index ? null : i)
    }
  }
  const onPointerUp = (e: PointerEvent) => {
    const p = press.current
    press.current = null
    setOver(null)
    if (!p) return
    if (!p.dragging) return tap(p.index)
    const to = indexAt(e.clientX, e.clientY)
    if (to !== null && to !== p.index) onMove(p.index, to)
    setPicked(null)
  }
  const onKeyDown = (i: number) => (e: KeyboardEvent) => {
    if (picked !== i) return
    const to = e.key === 'ArrowLeft' ? i - 1 : e.key === 'ArrowRight' ? i + 1 : null
    if (to === null || to < 0 || to >= cards.length) return
    e.preventDefault()
    onMove(i, to)
    setPicked(to)
  }

  return (
    <div className="hand-cards">
      <div className="hand-cards-row" role="group" aria-label="Your cards">
        {cards.map((c, i) => (
          <button
            key={c}
            type="button"
            data-card-index={i}
            className={[
              'hand-card',
              picked === i && 'hand-card-picked',
              over === i && 'hand-card-over',
              shown?.includes(c) && 'shown-card',
            ]
              .filter(Boolean)
              .join(' ')}
            aria-pressed={picked === i}
            onPointerDown={onPointerDown(i)}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={() => {
              press.current = null
              setOver(null)
            }}
            onKeyDown={onKeyDown(i)}
            // Keyboard activation (Enter/Space) arrives as a click with no pointer.
            onClick={(e) => e.detail === 0 && tap(i)}
          >
            <Card code={c} />
          </button>
        ))}
      </div>
      {cards.length > 2 && (
        <button type="button" className="btn btn-small btn-link hand-sort" onClick={onSort}>
          {SORT_LABEL[nextSort]}
        </button>
      )}
    </div>
  )
}
