import { useCallback, useState } from 'react'

// Players can rearrange their own hole cards. The order is only for them:
// it's kept in this browser per room, survives reloads, and resets with the
// next hand's deal.

export type SortMode = 'rank' | 'suit' | 'dealt'

const RANKS = '23456789TJQKA'
// Alternating colors, so neighbouring suits are easy to tell apart.
const SUIT_ORDER = 'shcd'

const byRank = (a: string, b: string) => RANKS.indexOf(b[0]) - RANKS.indexOf(a[0]) || SUIT_ORDER.indexOf(a[1]) - SUIT_ORDER.indexOf(b[1])
const bySuit = (a: string, b: string) => SUIT_ORDER.indexOf(a[1]) - SUIT_ORDER.indexOf(b[1]) || RANKS.indexOf(b[0]) - RANKS.indexOf(a[0])

/** The cards sorted by mode: high to low, grouped by suit, or as dealt. */
export function sortCards(dealt: string[], mode: SortMode): string[] {
  if (mode === 'dealt') return [...dealt]
  return [...dealt].sort(mode === 'rank' ? byRank : bySuit)
}

/** What the Sort button does next, given the current order. */
export function nextSort(dealt: string[], current: string[]): SortMode {
  const same = (mode: SortMode) => sortCards(dealt, mode).join() === current.join()
  if (same('rank')) return 'suit'
  if (same('suit') && !same('dealt')) return 'dealt'
  return 'rank'
}

/**
 * The hole cards in the saved order. Cards the saved order doesn't know
 * (a new deal) keep their dealt position at the end.
 */
export function applyOrder(dealt: string[], saved: string[] | undefined): string[] {
  if (!saved) return [...dealt]
  const kept = saved.filter((c) => dealt.includes(c))
  return [...kept, ...dealt.filter((c) => !kept.includes(c))]
}

/** Moves the card at from to index to, shifting the ones between. */
export function moveCard(cards: string[], from: number, to: number): string[] {
  const out = [...cards]
  const [c] = out.splice(from, 1)
  out.splice(to, 0, c)
  return out
}

export function swapCards(cards: string[], i: number, j: number): string[] {
  const out = [...cards]
  ;[out[i], out[j]] = [out[j], out[i]]
  return out
}

interface Saved {
  hand: number
  cards: string[]
}

const key = (room: string) => `cardOrder:${room}`

function load(room: string, hand: number): string[] | undefined {
  try {
    const s = JSON.parse(localStorage.getItem(key(room)) ?? 'null') as Saved | null
    return s && s.hand === hand && Array.isArray(s.cards) ? s.cards : undefined
  } catch {
    return undefined
  }
}

function save(room: string, hand: number, cards: string[]) {
  try {
    // One entry per room, replaced every hand, so nothing piles up.
    localStorage.setItem(key(room), JSON.stringify({ hand, cards } satisfies Saved))
  } catch {
    // Storage blocked: the order just won't survive a reload.
  }
}

/** Your hole cards in your chosen order, and ways to change it. */
export function useCardOrder(room: string, hand: number, dealt: string[]) {
  const [saved, setSaved] = useState<{ hand: number; cards: string[] | undefined }>(() => ({ hand, cards: load(room, hand) }))
  // A new hand starts from the dealt order (or whatever this browser saved
  // for it, after a reload).
  const current = saved.hand === hand ? saved.cards : load(room, hand)
  const cards = applyOrder(dealt, current)

  const set = useCallback(
    (next: string[]) => {
      save(room, hand, next)
      setSaved({ hand, cards: next })
    },
    [room, hand],
  )

  return {
    cards,
    move: (from: number, to: number) => set(moveCard(cards, from, to)),
    swap: (i: number, j: number) => set(swapCards(cards, i, j)),
    sort: () => set(sortCards(dealt, nextSort(dealt, cards))),
    nextSort: nextSort(dealt, cards),
  }
}
