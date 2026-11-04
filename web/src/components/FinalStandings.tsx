import type { ClientMsg, State } from '../protocol/types.ts'

const fmt = (n: number) => n.toLocaleString()

/**
 * The end of the game: everyone ranked by profit (stack − starting stack −
 * debt), front and centre, with what's behind each number and the host's
 * button to play again.
 */
export function FinalStandings({ state, send }: { state: State; send: (m: ClientMsg) => void }) {
  const isHost = state.you === state.hostId
  const host = state.players.find((p) => p.id === state.hostId)
  const rows = state.leaderboard.map((s) => ({
    ...s,
    // Players on the same profit share a place.
    rank: 1 + state.leaderboard.filter((o) => o.profit > s.profit).length,
    player: state.players.find((p) => p.id === s.id),
  }))

  return (
    <section className="final" aria-labelledby="final-title">
      <h2 id="final-title">Final standings</h2>
      <p className="final-note muted">Profit = final stack − starting stack − debt</p>
      <ol className="final-list">
        {rows.map((r) => {
          const p = r.player
          const winner = r.rank === 1 && r.profit > 0
          const details = p && [
            `${fmt(p.stack)} chips`,
            p.loans > 0 && `${p.loans} loan${p.loans === 1 ? '' : 's'}`,
            p.debt > 0 && `${fmt(p.debt)} debt`,
          ]
          return (
            <li
              key={r.id}
              className={['final-row', winner && 'final-winner', r.id === state.you && 'final-me'].filter(Boolean).join(' ')}
              aria-label={`${r.rank}. ${r.name}`}
            >
              <span className="final-rank" aria-hidden>
                {winner ? '🏆' : r.rank}
              </span>
              <span className="final-who">
                <span className="final-name">
                  {r.name}
                  {r.id === state.you && <span className="tag">you</span>}
                  {r.kicked && <span className="tag tag-muted">removed</span>}
                </span>
                {details && <span className="final-detail">{details.filter(Boolean).join(' · ')}</span>}
              </span>
              <span className={`final-profit ${r.profit > 0 ? 'profit-up' : r.profit < 0 ? 'profit-down' : 'profit-even'}`}>
                {r.profit > 0 ? `+${fmt(r.profit)}` : r.profit < 0 ? `−${fmt(-r.profit)}` : 'even'}
              </span>
            </li>
          )
        })}
      </ol>
      <div className="final-actions">
        {isHost ? (
          <button className="btn btn-primary" onClick={() => send({ type: 'rematch' })}>
            New game with same players
          </button>
        ) : (
          <p className="muted">{host?.name ?? 'The host'} (the host) can start a new game with everyone here.</p>
        )}
      </div>
    </section>
  )
}
