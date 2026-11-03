import { useEffect, useState } from 'react'

/** The current time, re-rendering every `every` ms (0 = don't tick). */
// eslint-disable-next-line react-refresh/only-export-components
export function useNow(every = 250): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!every) return
    const id = setInterval(() => setNow(Date.now()), every)
    return () => clearInterval(id)
  }, [every])
  return now
}

function secondsLeft(deadline: number, now: number, clockOffset: number) {
  return Math.max(0, Math.ceil((deadline - (now + clockOffset)) / 1000))
}

/** Seconds left until a server-time deadline. */
export function Countdown({ deadline, clockOffset }: { deadline: number; clockOffset: number }) {
  const left = secondsLeft(deadline, useNow(), clockOffset)
  return (
    <span className={`countdown${left <= 5 ? ' countdown-low' : ''}`} aria-label={`${left} seconds left`}>
      {left}s
    </span>
  )
}

/** A ring that empties as the player's time runs out. */
export function TimerRing({ deadline, total, clockOffset }: { deadline: number; total: number; clockOffset: number }) {
  const now = useNow()
  const left = secondsLeft(deadline, now, clockOffset)
  const fraction = Math.min(1, Math.max(0, (deadline - (now + clockOffset)) / total))
  const r = 11
  const circumference = 2 * Math.PI * r
  return (
    <span className={`timer-ring${left <= 5 ? ' countdown-low' : ''}`} aria-label={`${left} seconds left`}>
      <svg viewBox="0 0 28 28" width="28" height="28" aria-hidden="true">
        <circle cx="14" cy="14" r={r} className="timer-track" />
        <circle
          cx="14"
          cy="14"
          r={r}
          className="timer-fill"
          strokeDasharray={circumference}
          strokeDashoffset={circumference * (1 - fraction)}
        />
      </svg>
      <span className="timer-num">{left}</span>
    </span>
  )
}
