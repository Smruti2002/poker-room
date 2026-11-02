import { useEffect, useState } from 'react'
import { getRoom } from '../api.ts'
import { Lobby } from '../components/Lobby.tsx'
import { Table } from '../components/Table.tsx'
import { useRoom } from '../useRoom.ts'

export function RoomPage({ code }: { code: string }) {
  const room = useRoom(code)
  const [exists, setExists] = useState<boolean | null>(null)

  useEffect(() => {
    getRoom(code)
      .then((r) => setExists(r !== null))
      .catch(() => setExists(true)) // let the WebSocket report real problems
  }, [code])

  if (exists === false) {
    return (
      <main className="home">
        <h1>Room not found</h1>
        <p className="muted">The room {code.toUpperCase()} doesn't exist. Check the link, or start a new room.</p>
        <a className="btn btn-primary" href="/">
          New room
        </a>
      </main>
    )
  }

  if (!room.hasSession && !room.state) {
    return <JoinForm code={code} error={room.joinError} busy={room.status === 'connecting'} onJoin={room.join} />
  }

  const s = room.state
  return (
    <div className="room">
      <nav className="topbar">
        <a href="/" className="brand">
          Poker Room
        </a>
        <span className="room-code">Room {code.toUpperCase()}</span>
        {room.status !== 'open' && <span className="tag tag-muted">{room.status}…</span>}
      </nav>
      {room.error && (
        <div className="toast" role="alert" onClick={room.dismissError}>
          {room.error}
        </div>
      )}
      {!s ? (
        <p className="muted center">Connecting…</p>
      ) : s.phase === 'lobby' ? (
        <Lobby state={s} onSave={room.updateSettings} onStart={() => room.send({ type: 'start' })} />
      ) : (
        <Table state={s} clockOffset={room.clockOffset} onAct={room.act} send={room.send} />
      )}
    </div>
  )
}

function JoinForm({
  code,
  error,
  busy,
  onJoin,
}: {
  code: string
  error: string | null
  busy: boolean
  onJoin: (name: string) => void
}) {
  const [name, setName] = useState('')
  return (
    <main className="home">
      <h1>Join room {code.toUpperCase()}</h1>
      <form
        className="panel home-form"
        onSubmit={(e) => {
          e.preventDefault()
          onJoin(name.trim())
        }}
      >
        <label>
          <span>Your name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={20} autoFocus required />
        </label>
        <button className="btn btn-primary" disabled={busy || name.trim() === ''}>
          {busy ? 'Joining…' : 'Join'}
        </button>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
      </form>
    </main>
  )
}
