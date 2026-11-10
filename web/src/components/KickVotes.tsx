import type { ClientMsg, PlayerView, State } from '../protocol/types.ts'

/**
 * Starts a vote to remove a player. It passes once more than half of the
 * connected players (not counting them) say yes.
 */
export function KickButton({ p, state, send }: { p: PlayerView; state: State; send: (m: ClientMsg) => void }) {
  const open = state.kickVotes.some((v) => v.target === p.id)
  if (p.bot) return <RemoveBot p={p} state={state} send={send} />
  if (p.id === state.you || p.kicked || state.phase === 'ended' || open) return null
  const start = () => {
    if (window.confirm(`Start a vote to remove ${p.name}? They're removed if most players here agree.`)) {
      send({ type: 'voteKick', target: p.id, yes: true })
    }
  }
  return (
    <button className="btn btn-small btn-link btn-kick" onClick={start} aria-label={`Vote to remove ${p.name}`}>
      Kick
    </button>
  )
}

/** The votes in progress, with Yes / No for everyone but the target. */
export function KickVotes({ state, send }: { state: State; send: (m: ClientMsg) => void }) {
  if (state.kickVotes.length === 0) return null
  const name = (id: string) => state.players.find((p) => p.id === id)?.name ?? '?'
  return (
    <div className="kick-votes">
      {state.kickVotes.map((v) => {
        const tally = `${v.yes.length} of ${v.needed} yes${v.no.length ? `, ${v.no.length} no` : ''}`
        if (v.target === state.you) {
          return (
            <div key={v.target} className="banner banner-warn" role="status">
              The other players are voting on removing you ({tally}).
            </div>
          )
        }
        const mine = v.yes.includes(state.you) ? 'yes' : v.no.includes(state.you) ? 'no' : null
        const vote = (yes: boolean) => send({ type: 'voteKick', target: v.target, yes })
        return (
          <div key={v.target} className="banner" role="group" aria-label={`Vote to remove ${name(v.target)}`}>
            <span>
              Remove <strong>{name(v.target)}</strong>? {tally}
            </span>
            <span className="banner-actions">
              <button className={`btn btn-small${mine === 'yes' ? ' btn-primary' : ''}`} aria-pressed={mine === 'yes'} onClick={() => vote(true)}>
                Yes
              </button>
              <button className={`btn btn-small${mine === 'no' ? ' btn-primary' : ''}`} aria-pressed={mine === 'no'} onClick={() => vote(false)}>
                No
              </button>
              {v.startedBy === state.you && (
                <button className="btn btn-small btn-link" onClick={() => send({ type: 'cancelKick', target: v.target })}>
                  Cancel
                </button>
              )}
            </span>
          </div>
        )
      })}
    </div>
  )
}

/** The host can remove a bot at any time; mid-game it leaves like a kicked player. */
function RemoveBot({ p, state, send }: { p: PlayerView; state: State; send: (m: ClientMsg) => void }) {
  if (state.you !== state.hostId || p.kicked || state.phase === 'ended') return null
  const remove = () => {
    if (state.phase === 'lobby' || window.confirm(`Remove ${p.name}? It folds out of this hand and stays on the leaderboard.`)) {
      send({ type: 'removeBot', target: p.id })
    }
  }
  return (
    <button className="btn btn-small btn-link btn-kick" onClick={remove} aria-label={`Remove ${p.name}`}>
      Remove
    </button>
  )
}

/** The host seats a computer player that plays by itself. */
export function AddBot({ state, send }: { state: State; send: (m: ClientMsg) => void }) {
  const seated = state.players.filter((p) => !p.kicked).length
  if (state.you !== state.hostId || state.phase === 'ended' || seated >= state.settings.maxPlayers) return null
  return (
    <button className="btn btn-small" onClick={() => send({ type: 'addBot' })}>
      🤖 Add bot
    </button>
  )
}
