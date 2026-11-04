import { useEffect, useRef, useState } from 'react'
import type { Action, Legal } from '../protocol/types.ts'
import { ChipFace } from './BetChip.tsx'

/** Common bet sizes, in table units: how much to bet, or to raise by. */
const CHIP_AMOUNTS = [50, 100, 150, 250, 500]

interface Props {
  legal: Legal
  /** Units already committed on this street. */
  bet: number
  /** Chips per unit (2 for a Dacoit 6-card player). */
  mult: number
  /** Chips behind, to tell an all-in from a pot-limit cap. */
  stack: number
  onAct: (action: Action) => void
}

/** Fold / check / call / bet / raise controls for the player to act. */
export function ActionBar({ legal, bet, mult, stack, onAct }: Props) {
  const canRaise = legal.bet || legal.raise
  const clamp = (n: number) => Math.min(Math.max(n, legal.minTo), legal.maxTo)
  // The typed text is kept as-is so it can be cleared and retyped; it is
  // only validated when raising.
  const [text, setText] = useState(String(legal.minTo))
  const typed = /^\d+$/.test(text.trim()) ? Number(text.trim()) : NaN
  const valid = typed >= legal.minTo && typed <= legal.maxTo
  const raiseType = legal.bet ? 'bet' : 'raise'
  const allInOnly = canRaise && legal.minTo === legal.maxTo
  const amount = allInOnly ? legal.maxTo : typed

  // Bet chips: each amount is how much to bet, or how much to raise by on
  // top of the bet faced, in table units (a 6-card Dacoit player pays
  // double). Below the minimum a chip is unavailable. Above the maximum it
  // means all-in, unless the maximum is a pot limit, which caps it.
  const faced = bet + legal.callUnits
  const allInTo = bet + Math.floor(stack / mult)
  const capped = legal.maxTo < allInTo // pot-limit
  const chipTo = (amount: number) => faced + amount
  // Desktop shortcuts: F fold, C check/call, R jump to the raise box.
  const box = useRef<HTMLInputElement>(null)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = e.target instanceof Element && e.target.closest('input, textarea, select')
      if (e.metaKey || e.ctrlKey || e.altKey || typing) return
      const key = e.key.toLowerCase()
      if (key === 'f' && legal.fold) onAct({ type: 'fold' })
      else if (key === 'c' && legal.check) onAct({ type: 'check' })
      else if (key === 'c' && legal.call) onAct({ type: 'call' })
      else if (key === 'r' && box.current) {
        e.preventDefault()
        box.current.focus()
        box.current.select()
      } else return
      e.preventDefault()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [legal, onAct])


  return (
    <div className="action-bar" role="group" aria-label="Your action">
      <div className="action-main">
        {legal.fold && (
          <button className="btn btn-fold" onClick={() => onAct({ type: 'fold' })}>
            Fold
          </button>
        )}
        {legal.check && (
          <button className="btn" onClick={() => onAct({ type: 'check' })}>
            Check
          </button>
        )}
        {legal.call && (
          <button className="btn" onClick={() => onAct({ type: 'call' })}>
            Call {legal.callUnits * mult}
          </button>
        )}
        {canRaise && (
          <button
            className="btn btn-raise"
            disabled={!allInOnly && !valid}
            onClick={() => onAct({ type: raiseType, to: amount })}
          >
            {allInOnly
              ? `All-in ${amount * mult}`
              : valid
                ? `${legal.bet ? 'Bet' : 'Raise to'} ${amount * mult}`
                : legal.bet
                  ? 'Bet'
                  : 'Raise'}
          </button>
        )}
      </div>
      {canRaise && !allInOnly && (
        <div className="raise">
          <div className="bet-chips" role="group" aria-label="Bet chips">
            {CHIP_AMOUNTS.map((amount) => {
              const to = chipTo(amount)
              const off = to < legal.minTo || (capped && to > legal.maxTo)
              const target = clamp(to)
              const verb = legal.bet ? 'Chip: bet' : 'Chip: raise by'
              return (
                <button
                  key={amount}
                  type="button"
                  className={`bet-chip-btn${valid && typed === target && !off ? ' bet-chip-on' : ''}`}
                  disabled={off}
                  onClick={() => setText(String(target))}
                  aria-label={to > legal.maxTo && !capped ? `${verb} ${amount}: all-in` : `${verb} ${amount}`}
                >
                  <ChipFace label={String(amount)} color={String(amount)} />
                </button>
              )
            })}
            <button
              type="button"
              className={`bet-chip-btn${valid && typed === legal.maxTo ? ' bet-chip-on' : ''}`}
              onClick={() => setText(String(legal.maxTo))}
              aria-label={capped ? 'Chip: pot-limit maximum' : 'Chip: all-in'}
            >
              <ChipFace label={capped ? 'MAX' : 'ALL IN'} color="allin" />
            </button>
          </div>
          <div className="raise-amount">
            <input
              type="range"
              min={legal.minTo}
              max={legal.maxTo}
              value={valid ? typed : legal.minTo}
              onChange={(e) => setText(e.target.value)}
              aria-label={`${raiseType} amount`}
            />
            <input
              ref={box}
              type="text"
              inputMode="numeric"
              className={`amount${valid ? '' : ' amount-invalid'}`}
              value={text}
              onChange={(e) => setText(e.target.value.replace(/[^\d]/g, ''))}
              onBlur={() => {
                if (!valid && text.trim() !== '') setText(String(clamp(Number.isNaN(typed) ? legal.minTo : typed)))
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && valid) onAct({ type: raiseType, to: amount })
              }}
              aria-label={`${raiseType} to`}
              aria-invalid={!valid}
            />
          </div>
          <div className="shortcuts" aria-hidden="true">
            Keys: F fold · C {legal.check ? 'check' : 'call'} · R raise · Enter confirm
          </div>
          {!valid && (
            <div className="amount-hint" role="note">
              Enter {legal.minTo}–{legal.maxTo}
              {mult > 1 && ` (you pay ${mult}× in chips)`}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
