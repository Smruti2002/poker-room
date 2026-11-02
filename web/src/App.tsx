import { useEffect, useState } from 'react'

type ServerStatus = 'checking' | 'online' | 'offline'

export default function App() {
  const [status, setStatus] = useState<ServerStatus>('checking')

  useEffect(() => {
    let cancelled = false
    fetch('/healthz')
      .then((res) => {
        if (!cancelled) setStatus(res.ok ? 'online' : 'offline')
      })
      .catch(() => {
        if (!cancelled) setStatus('offline')
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <main className="home">
      <h1>Poker Room</h1>
      <p>Play-money poker with friends. Create a room, share the link, deal.</p>
      <p className={`status status-${status}`} role="status">
        Server: {status}
      </p>
    </main>
  )
}
