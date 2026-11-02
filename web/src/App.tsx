import { useEffect, useState } from 'react'
import { Home } from './pages/Home.tsx'
import { RoomPage } from './pages/RoomPage.tsx'

/** Tiny router: "/" is the home page, "/r/CODE" is a room. */
export default function App() {
  const [path, setPath] = useState(() => window.location.pathname)

  useEffect(() => {
    const onPop = () => setPath(window.location.pathname)
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  const navigate = (to: string) => {
    window.history.pushState(null, '', to)
    setPath(to)
  }

  const match = /^\/r\/([A-Za-z0-9]+)\/?$/.exec(path)
  if (match) return <RoomPage key={match[1]} code={match[1].toUpperCase()} />
  return <Home navigate={navigate} />
}
