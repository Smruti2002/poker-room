import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { applyOrder, moveCard, nextSort, sortCards, swapCards, useCardOrder } from './cardOrder.ts'

const dealt = ['7h', 'As', '7c', 'Kh']

describe('card order helpers', () => {
  it('sorts by rank (high first), by suit, or as dealt', () => {
    expect(sortCards(dealt, 'rank')).toEqual(['As', 'Kh', '7h', '7c'])
    expect(sortCards(dealt, 'suit')).toEqual(['As', 'Kh', '7h', '7c'])
    expect(sortCards(['2d', 'Tc', '9h', 'Js'], 'suit')).toEqual(['Js', '9h', 'Tc', '2d'])
    expect(sortCards(dealt, 'dealt')).toEqual(dealt)
  })

  it('cycles rank → suit → as dealt → rank', () => {
    const d = ['2d', 'Tc', '9h', 'Js']
    expect(nextSort(d, d)).toBe('rank')
    expect(nextSort(d, sortCards(d, 'rank'))).toBe('suit')
    expect(nextSort(d, sortCards(d, 'suit'))).toBe('dealt')
    expect(nextSort(d, ['Tc', '2d', '9h', 'Js'])).toBe('rank') // a custom order
  })

  it('applies a saved order and keeps unknown cards at the end', () => {
    expect(applyOrder(dealt, undefined)).toEqual(dealt)
    expect(applyOrder(dealt, ['Kh', '7h'])).toEqual(['Kh', '7h', 'As', '7c'])
    expect(applyOrder(dealt, ['Qd', 'As'])).toEqual(['As', '7h', '7c', 'Kh']) // Qd isn't held
  })

  it('moves and swaps', () => {
    expect(moveCard(dealt, 0, 3)).toEqual(['As', '7c', 'Kh', '7h'])
    expect(moveCard(dealt, 3, 1)).toEqual(['7h', 'Kh', 'As', '7c'])
    expect(swapCards(dealt, 0, 2)).toEqual(['7c', 'As', '7h', 'Kh'])
  })
})

describe('useCardOrder', () => {
  afterEach(() => localStorage.clear())

  it('keeps your order through a reload and resets it for the next hand', () => {
    const first = renderHook(() => useCardOrder('ROOM01', 3, dealt))
    act(() => first.result.current.swap(0, 3))
    expect(first.result.current.cards).toEqual(['Kh', 'As', '7c', '7h'])
    first.unmount()

    // Reload: same hand, same order.
    const again = renderHook(({ hand, cards }) => useCardOrder('ROOM01', hand, cards), { initialProps: { hand: 3, cards: dealt } })
    expect(again.result.current.cards).toEqual(['Kh', 'As', '7c', '7h'])

    // Next hand: as dealt.
    const next = ['2c', '3d', '4h', '5s']
    again.rerender({ hand: 4, cards: next })
    expect(again.result.current.cards).toEqual(next)
  })

  it('sorts in turn', () => {
    const { result } = renderHook(() => useCardOrder('ROOM02', 1, dealt))
    expect(result.current.nextSort).toBe('rank')
    act(() => result.current.sort())
    expect(result.current.cards).toEqual(['As', 'Kh', '7h', '7c'])
    expect(result.current.nextSort).toBe('suit')
  })

  it('works when storage is blocked', () => {
    const real = Storage.prototype.setItem
    Storage.prototype.setItem = () => {
      throw new Error('blocked')
    }
    try {
      const { result } = renderHook(() => useCardOrder('ROOM03', 1, dealt))
      act(() => result.current.swap(0, 1))
      expect(result.current.cards).toEqual(['As', '7h', '7c', 'Kh'])
    } finally {
      Storage.prototype.setItem = real
    }
  })
})
