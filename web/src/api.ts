export interface CreatedRoom {
  code: string
  token: string
}

export interface RoomSummary {
  code: string
  phase: string
  players: number
  maxPlayers: number
}

async function errorMessage(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string }
    return body.error ?? `Request failed (${res.status})`
  } catch {
    return `Request failed (${res.status})`
  }
}

export async function createRoom(name: string): Promise<CreatedRoom> {
  const res = await fetch('/api/rooms', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (!res.ok) throw new Error(await errorMessage(res))
  return (await res.json()) as CreatedRoom
}

/** Returns null when the room doesn't exist. */
export async function getRoom(code: string): Promise<RoomSummary | null> {
  const res = await fetch(`/api/rooms/${encodeURIComponent(code)}`)
  if (res.status === 404) return null
  if (!res.ok) throw new Error(await errorMessage(res))
  return (await res.json()) as RoomSummary
}

// Session tokens live in localStorage so a refresh or reconnect keeps the
// player's seat. Storage can be unavailable (private mode), so never throw.
const tokenKey = (code: string) => `poker-room:token:${code.toUpperCase()}`

export function loadToken(code: string): string | null {
  try {
    return localStorage.getItem(tokenKey(code))
  } catch {
    return null
  }
}

export function saveToken(code: string, token: string) {
  try {
    localStorage.setItem(tokenKey(code), token)
  } catch {
    // ignore
  }
}

export function clearToken(code: string) {
  try {
    localStorage.removeItem(tokenKey(code))
  } catch {
    // ignore
  }
}

/**
 * Downloads a finished game's every hand as a JSON file. The session token
 * proves the player was in the room; it goes in a header, not the URL.
 */
export async function downloadHistory(code: string): Promise<void> {
  const token = loadToken(code)
  const res = await fetch(`/api/rooms/${encodeURIComponent(code)}/history`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!res.ok) throw new Error(await errorMessage(res))
  const url = URL.createObjectURL(await res.blob())
  const a = document.createElement('a')
  a.href = url
  a.download = `poker-room-${code}.json`
  document.body.append(a)
  a.click()
  a.remove()
  // Give the browser a moment to start the download before letting go.
  setTimeout(() => URL.revokeObjectURL(url), 10_000)
}
