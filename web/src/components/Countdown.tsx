import { useEffect, useState } from 'react'

/** Seconds left until a server-time deadline. */
export function Countdown({ deadline, clockOffset }: { deadline: number; clockOffset: number }) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 250)
    return () => clearInterval(id)
  }, [])
  const left = Math.max(0, Math.ceil((deadline - (now + clockOffset)) / 1000))
  return (
    <span className={`countdown${left <= 5 ? ' countdown-low' : ''}`} aria-label={`${left} seconds left`}>
      {left}s
    </span>
  )
}
