import { useCallback, useEffect, useRef, useState } from 'react'
import { clearToken, getRoom, loadToken, saveToken } from './api.ts'
import type { Action, ClientMsg, HistoryHand, ServerMsg, Settings, State } from './protocol/types.ts'

export type ConnStatus = 'idle' | 'connecting' | 'open' | 'reconnecting'

export interface RoomConn {
  state: State | null
  status: ConnStatus
  error: string | null
  /** Set when joining failed (e.g. name taken); the join form shows it. */
  joinError: string | null
  /** True when the room no longer exists, e.g. after a server restart. */
  gone: boolean
  /** True once we hold a session token for this room. */
  hasSession: boolean
  /** Server clock minus local clock, for countdowns. */
  clockOffset: number
  join: (name: string) => void
  send: (msg: ClientMsg) => void
  act: (action: Action) => void
  updateSettings: (settings: Settings) => void
  dismissError: () => void
  /** Recent hands, once requested with loadHistory. */
  history: HistoryHand[] | null
  loadHistory: () => void
  chat: (text: string) => void
  react: (emoji: string) => void
}

function wsURL(code: string) {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws/${encodeURIComponent(code)}`
}

/**
 * Keeps a WebSocket open to the room. With a saved token it resumes the
 * player's seat automatically, reconnecting with backoff if the connection
 * drops. Without one it waits for join(name).
 */
export function useRoom(code: string): RoomConn {
  const [state, setState] = useState<State | null>(null)
  const [status, setStatus] = useState<ConnStatus>('idle')
  const [error, setError] = useState<string | null>(null)
  const [joinError, setJoinError] = useState<string | null>(null)
  const [token, setToken] = useState<string | null>(() => loadToken(code))
  const [clockOffset, setClockOffset] = useState(0)
  const [pendingName, setPendingName] = useState<string | null>(null)
  const [gone, setGone] = useState(false)
  const [history, setHistory] = useState<HistoryHand[] | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    const hello: ClientMsg | null = token
      ? { type: 'resume', token }
      : pendingName
        ? { type: 'join', name: pendingName }
        : null
    if (!hello) return
    let stopped = false
    let attempt = 0
    let retry: ReturnType<typeof setTimeout> | undefined
    let welcomed = false

    const connect = () => {
      setStatus(attempt === 0 ? 'connecting' : 'reconnecting')
      const ws = new WebSocket(wsURL(code))
      wsRef.current = ws
      ws.onopen = () => ws.send(JSON.stringify(hello))
      ws.onmessage = (ev) => {
        const msg = JSON.parse(ev.data as string) as ServerMsg
        switch (msg.type) {
          case 'welcome':
            welcomed = true
            attempt = 0
            setStatus('open')
            if (msg.token) {
              saveToken(code, msg.token)
              setToken(msg.token)
            }
            break
          case 'state':
            if (msg.state) {
              setState(msg.state)
              setClockOffset(msg.state.now - Date.now())
            }
            break
          case 'history':
            setHistory(msg.history ?? [])
            break
          case 'error':
            if (!welcomed) {
              // The handshake was refused: bad token or bad name.
              stopped = true
              if (hello.type === 'resume') {
                clearToken(code)
                setToken(null)
              } else {
                setJoinError(msg.error ?? 'Could not join')
                setPendingName(null)
              }
              setStatus('idle')
            } else {
              setError(msg.error ?? 'Something went wrong')
            }
            break
        }
      }
      ws.onclose = () => {
        if (wsRef.current === ws) wsRef.current = null
        if (stopped) return
        // Lost connection: retry with backoff, resuming by token. If the
        // room itself is gone (e.g. the server restarted), stop retrying.
        attempt++
        setStatus('reconnecting')
        retry = setTimeout(async () => {
          if (attempt >= 2 && (await getRoom(code).catch(() => undefined)) === null) {
            stopped = true
            setStatus('idle')
            setGone(true)
            return
          }
          if (!stopped) connect()
        }, Math.min(500 * 2 ** attempt, 8000))
      }
    }
    connect()
    return () => {
      stopped = true
      clearTimeout(retry)
      wsRef.current?.close()
    }
  }, [code, token, pendingName])

  const send = useCallback((msg: ClientMsg) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg))
    } else {
      setError('Not connected, retrying…')
    }
  }, [])

  return {
    state,
    status,
    error,
    joinError,
    gone,
    hasSession: token !== null,
    clockOffset,
    join: (name: string) => {
      setJoinError(null)
      setPendingName(name)
    },
    send,
    act: (action: Action) => send({ type: 'action', action }),
    updateSettings: (settings: Settings) => send({ type: 'updateSettings', settings }),
    dismissError: () => setError(null),
    history,
    loadHistory: () => send({ type: 'history' }),
    chat: (text: string) => send({ type: 'chat', text }),
    react: (emoji: string) => send({ type: 'react', emoji }),
  }
}
