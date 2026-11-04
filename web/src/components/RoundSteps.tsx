import { roundSteps } from '../roundSteps.ts'
import type { HandView } from '../protocol/types.ts'

/**
 * Where the hand is: a step list on the felt with the current step
 * highlighted. On a phone only the current step is spelled out ("Turn ·
 * 3 of 5"), with a dot for each step.
 */
export function RoundSteps({ hand }: { hand: HandView }) {
  const steps = roundSteps(hand)
  const at = steps.findIndex((s) => s.state === 'current')
  const current = steps[at]
  return (
    <div className="round-steps">
      <ol className="round-steps-list" aria-label="Hand progress">
        {steps.map((s) => (
          <li key={s.label} className={`round-step round-step-${s.state}`} aria-current={s.state === 'current' ? 'step' : undefined}>
            {s.state === 'done' && <span aria-hidden>✓ </span>}
            {s.label}
          </li>
        ))}
      </ol>
      {current && (
        // Re-keyed on every change so the pulse plays when the step moves on.
        <div className="round-steps-compact" key={current.label} aria-hidden>
          <span className="round-steps-now">{current.label}</span>
          <span className="round-steps-count">
            {at + 1} of {steps.length}
          </span>
        </div>
      )}
    </div>
  )
}
