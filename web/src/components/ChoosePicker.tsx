import type { Action } from '../protocol/types.ts'

/**
 * Dacoit's choice before the deal: 4 cards at normal prices, or 6 cards
 * paying double for every bet (docs/rules.md §4.3).
 */
export function ChoosePicker({ max, onAct }: { max: number; onAct: (a: Action) => void }) {
  const sixAllowed = max >= 6
  return (
    <div className="choose-picker" role="group" aria-label="Choose your cards">
      <p className="show-hint">How many cards do you want? You'll play exactly two of them.</p>
      <div className="choose-buttons">
        <button className="btn choose-btn" onClick={() => onAct({ type: 'choose', count: 4 })}>
          <span className="choose-count">4 cards</span>
          <span className="choose-note">Normal bets</span>
        </button>
        <button
          className="btn choose-btn choose-six"
          disabled={!sixAllowed}
          onClick={() => onAct({ type: 'choose', count: 6 })}
        >
          <span className="choose-count">6 cards</span>
          <span className="choose-note">{sixAllowed ? 'Every bet costs you double' : 'Not available'}</span>
        </button>
      </div>
      {!sixAllowed && (
        <p className="muted small">Not enough cards left in the deck (or chips) for another 6-card hand.</p>
      )}
    </div>
  )
}
