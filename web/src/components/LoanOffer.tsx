import { useState } from 'react'
import type { ClientMsg, PlayerView, Settings } from '../protocol/types.ts'

interface Props {
  me: PlayerView
  settings: Settings
  playing: boolean
  send: (m: ClientMsg) => void
}

/**
 * The loan offer. Broke players see it straight away; players who are only
 * running low (below 500 chips, between hands) get a button that opens it,
 * so nobody borrows by accident.
 */
export function LoanOffer({ me, settings, playing, send }: Props) {
  const [open, setOpen] = useState(false)
  const broke = me.broke && playing
  if (!broke && !me.canBorrow) return null

  if (!broke && !open) {
    return (
      <button className="btn btn-small loan-early" onClick={() => setOpen(true)}>
        Low on chips? Take a loan
      </button>
    )
  }

  const debt =
    me.loans < settings.penalizedLoans
      ? ` (adds ${settings.loanAmount + settings.loanPenalty} to your debt, including the ${settings.loanPenalty} penalty)`
      : ' (no penalty)'
  return (
    <div className="panel loan">
      <p>
        {broke ? "You're out of chips. " : `You have ${me.stack} chips. `}
        Borrow {settings.loanAmount}
        {debt}?
      </p>
      <div className="loan-actions">
        <button
          className="btn btn-primary"
          onClick={() => {
            send({ type: 'takeLoan' })
            setOpen(false)
          }}
        >
          Take loan
        </button>
        {!broke && (
          <button className="btn" onClick={() => setOpen(false)}>
            Not now
          </button>
        )}
      </div>
    </div>
  )
}
