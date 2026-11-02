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
  const [to, setTo] = useState(legal.minTo)
  const amount = Math.min(Math.max(to, legal.minTo), legal.maxTo)
  const raiseType = legal.bet ? 'bet' : 'raise'
  const allInOnly = canRaise && legal.minTo === legal.maxTo

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
        <div className="raise">
          {!allInOnly && (
            <>
              <div className="presets">
                {presets.map(([label, value]) => (
                  <button
                    key={label}
                    type="button"
                    className="btn btn-small"
                    onClick={() => setTo(Math.min(Math.max(value, legal.minTo), legal.maxTo))}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <input
                type="range"
                min={legal.minTo}
                max={legal.maxTo}
                value={amount}
                onChange={(e) => setTo(Number(e.target.value))}
                aria-label={`${raiseType} amount`}
              />
              <input
                type="number"
                className="amount"
                min={legal.minTo}
                max={legal.maxTo}
                value={amount}
                onChange={(e) => setTo(Number(e.target.value))}
                aria-label={`${raiseType} to`}
              />
            </>
          )}
          <button className="btn btn-raise" onClick={() => onAct({ type: raiseType, to: amount })}>
            {allInOnly ? `All-in ${amount * mult}` : `${legal.bet ? 'Bet' : 'Raise to'} ${amount * mult}`}
          </button>
        </div>
      )}
    </div>
  )
}
