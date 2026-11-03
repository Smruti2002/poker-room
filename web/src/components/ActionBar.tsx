import { useState } from 'react'
import type { Action, Legal } from '../protocol/types.ts'

interface Props {
  legal: Legal
  /** Units already committed on this street. */
  bet: number
  /** Chips per unit (2 for a Dacoit 6-card player). */
  mult: number
  pot: number
  onAct: (action: Action) => void
}

/** Fold / check / call / bet / raise controls for the player to act. */
export function ActionBar({ legal, bet, mult, pot, onAct }: Props) {
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

  // Quick sizes, all clamped to what's legal. A pot-sized raise first calls,
  // then raises by the pot after the call (docs/rules.md §4.0); the pot is in
  // chips and the raise in units, as in pot-limit Dacoit (§4.3).
  const called = bet + legal.callUnits
  const potAfterCall = pot + legal.callUnits * mult
  const presets: [string, number][] = [
    ['Min', legal.minTo],
    ['½ pot', called + Math.floor(potAfterCall / 2)],
    ['Pot', called + potAfterCall],
    ['All-in', legal.maxTo],
  ]

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
          <div className="presets">
            {presets.map(([label, value]) => (
              <button key={label} type="button" className="btn btn-small" onClick={() => setText(String(clamp(value)))}>
                {label}
              </button>
            ))}
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
