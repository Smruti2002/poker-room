import { useNow } from './useNow.ts'

/** How long before closing the notice appears. */
const NOTICE_MS = 60 * 60 * 1000

/**
 * Rooms close a fixed time after they're created (12 hours by default),
 * however busy they are. In the last hour the room says when, once: a
 * closing time, not a countdown.
 */
export function ClosingNotice({ closesAt, clockOffset }: { closesAt?: number; clockOffset: number }) {
  // Checked once a minute, just to know when the last hour starts.
  const now = useNow(closesAt ? 60_000 : 0) + clockOffset
  if (!closesAt || closesAt - now > NOTICE_MS) return null
  const at = new Date(closesAt - clockOffset).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return (
    <div className="closing-notice" role="status">
      This room closes at {at}. Start a new room to keep playing after that.
    </div>
  )
}
