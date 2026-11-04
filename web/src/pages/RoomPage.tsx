import { useEffect, useState } from 'react'
import { getRoom, saveToken } from '../api.ts'
import { Drawer, type DrawerTab } from '../components/Drawer.tsx'
import { KickVotes } from '../components/KickVotes.tsx'
import { Lobby } from '../components/Lobby.tsx'
import { Table } from '../components/Table.tsx'
import { useGameSounds } from '../gameSounds.ts'
import { usePrefs } from '../prefs.ts'
import type { State } from '../protocol/types.ts'
import { useRoom } from '../useRoom.ts'

export function RoomPage({ code }: { code: string }) {
  const room = useRoom(code)
  const [exists, setExists] = useState<boolean | null>(null)
  const [prefs, setPrefs] = usePrefs()
  const [drawer, setDrawer] = useState<DrawerTab | null>(null)
  // Unread chat: messages from others newer than the last one seen. While
  // the chat is open everything counts as seen.
  const [seenChat, setSeenChat] = useState(0)
  const lastChat = room.state?.chat.at(-1)?.id ?? 0
  const seen = drawer === 'chat' ? lastChat : seenChat
  const unread = (room.state?.chat ?? []).filter((l) => l.id > seen && l.playerId !== room.state?.you).length
  const openDrawer = (next: DrawerTab | null) => {
    if (drawer === 'chat') setSeenChat(lastChat)
    setDrawer(next)
  }
  useGameSounds(room.state, prefs)

  // The host started a new game with the same players: move there, already
  // seated. Replace this page in the history so Back doesn't bounce here.
  const next = room.state?.nextRoom
  useEffect(() => {
    if (!next?.token) return
    saveToken(next.code, next.token)
    window.history.replaceState(null, '', `/r/${next.code}`)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }, [next?.code, next?.token])

  useEffect(() => {
    getRoom(code)
      .then((r) => setExists(r !== null))
      .catch(() => setExists(true)) // let the WebSocket report real problems
  }, [code])

  if (room.removed) {
    return (
      <main className="home">
        <h1>You were removed</h1>
        <p className="muted">The other players in room {code.toUpperCase()} voted to remove you, so you can't rejoin it.</p>
        <a className="btn btn-primary" href="/">
          New room
        </a>
      </main>
    )
  }

  if (exists === false || room.gone) {
    return (
      <main className="home">
        <h1>Room not found</h1>
        <p className="muted">The room {code.toUpperCase()} doesn't exist — it may have ended, or the server restarted. Check the link, or start a new room.</p>
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
        {s && (
          <span className="topbar-actions">
            <button className="btn btn-small" onClick={() => openDrawer('chat')} aria-label={`Chat${unread ? ` (${unread} unread)` : ''}`}>
              💬{unread > 0 && <span className="badge">{unread}</span>}
            </button>
            <button className="btn btn-small" onClick={() => openDrawer('history')} aria-label="Hand history">
              🕘
            </button>
            <button className="btn btn-small" onClick={() => openDrawer('options')} aria-label="Options">
              ⚙︎
            </button>
          </span>
        )}
      </nav>
      {s && <HostAway state={s} onClaim={() => room.send({ type: 'claimHost' })} />}
      {s && <KickVotes state={s} send={room.send} />}
      {room.error && (
        <div className="toast" role="alert" onClick={room.dismissError}>
          {room.error}
        </div>
      )}
      {!s ? (
        <p className="muted center">Connecting…</p>
      ) : s.phase === 'lobby' ? (
        <Lobby state={s} onSave={room.updateSettings} onStart={() => room.send({ type: 'start' })} send={room.send} />
      ) : (
        <Table state={s} clockOffset={room.clockOffset} onAct={room.act} send={room.send} />
      )}
      {s && drawer && (
        <Drawer
          tab={drawer}
          onTab={openDrawer}
          onClose={() => openDrawer(null)}
          state={s}
          history={room.history}
          loadHistory={room.loadHistory}
          chat={room.chat}
          react={room.react}
          prefs={prefs}
          setPrefs={setPrefs}
        />
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

/**
 * When the host is offline nobody can start, skip or end the game, so anyone
 * still here can take over. There's no timer: it's offered straight away.
 */
function HostAway({ state, onClaim }: { state: State; onClaim: () => void }) {
  const host = state.players.find((p) => p.id === state.hostId)
  if (!host || host.connected || state.you === state.hostId || state.phase === 'ended') return null
  return (
    <div className="host-away" role="status">
      <span>{host.name} (the host) is offline.</span>
      <button className="btn btn-small btn-primary" onClick={onClaim}>
        Become host
      </button>
    </div>
  )
}
