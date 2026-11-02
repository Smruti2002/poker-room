import { type FormEvent, useState } from 'react'
import { createRoom, saveToken } from '../api.ts'

export function Home({ navigate }: { navigate: (path: string) => void }) {
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const room = await createRoom(name.trim())
      saveToken(room.code, room.token)
      navigate(`/r/${room.code}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create the room')
      setBusy(false)
    }
  }

  return (
    <main className="home">
      <h1>Poker Room</h1>
      <p className="muted">
        Play-money poker with friends. Create a room, share the link, and take turns as dealer picking the
        game.
      </p>
      <form className="panel home-form" onSubmit={submit}>
        <label>
          <span>Your name</span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={20}
            autoFocus
            required
            placeholder="e.g. Smruti"
          />
        </label>
        <button className="btn btn-primary" disabled={busy || name.trim() === ''}>
          {busy ? 'Creating…' : 'Create room'}
        </button>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
      </form>
      <p className="muted small">Got a link from a friend? Just open it.</p>
    </main>
  )
}
