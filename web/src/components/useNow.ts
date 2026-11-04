import { useEffect, useState } from 'react'

/** The current time, re-rendering every `every` ms (0 = don't tick). */
export function useNow(every = 250): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!every) return
    const id = setInterval(() => setNow(Date.now()), every)
    return () => clearInterval(id)
  }, [every])
  return now
}
