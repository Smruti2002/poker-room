import { useEffect, useRef } from 'react'
import type { Prefs } from './prefs.ts'
import type { State } from './protocol/types.ts'
import { latestCallout } from './callouts.ts'
import { sounds } from './sound.ts'

/**
 * Plays sounds (and vibrates) for game moments: your turn, chips going in,
 * and winning a hand. Each fires once per moment, not on every state update.
 */
export function useGameSounds(state: State | null, prefs: Prefs) {
  const last = useRef({ turn: '', pot: 0, won: '', raise: '' })
  useEffect(() => {
    const hand = state?.hand
    if (!state || !hand) return
    const l = last.current
    const me = state.players.find((p) => p.id === state.you)

    // Your turn (including reveals, choices and splits).
    const turnKey = hand.legal ? `${hand.number}-${hand.street}-${hand.currentBet}-${hand.toAct}` : ''
    if (turnKey && turnKey !== l.turn) {
      if (prefs.sound) sounds.turn()
      if (prefs.vibrate) navigator.vibrate?.(150)
    }
    l.turn = turnKey

    // Someone else bet or raised: a distinct sound instead of the chip click.
    const latest = latestCallout(state)
    const raiseKey = latest && latest.player.id !== state.you ? `${hand.number}-${hand.street}-${latest.player.id}-${latest.c.to}` : ''
    const raised = raiseKey !== '' && raiseKey !== l.raise
    if (raised && prefs.sound) sounds.raise()
    l.raise = raiseKey || l.raise

    // Chips into the pot.
    if (hand.pot > l.pot && !hand.done && !raised && prefs.sound) sounds.chip()
    l.pot = hand.done ? 0 : hand.pot

    // You won something.
    const wonKey = hand.done && (me?.won ?? 0) > 0 ? String(hand.number) : ''
    if (wonKey && wonKey !== l.won) {
      if (prefs.sound) sounds.win()
      if (prefs.vibrate) navigator.vibrate?.([80, 60, 80])
    }
    l.won = wonKey || l.won
  }, [state, prefs])
}
