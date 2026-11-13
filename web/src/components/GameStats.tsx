import type { GameStats as Stats, PlayerStats, State } from '../protocol/types.ts'
import { variantName } from '../variants.ts'

const fmt = (n: number) => n.toLocaleString()
const signed = (n: number) => (n > 0 ? `+${fmt(n)}` : n < 0 ? `−${fmt(-n)}` : '0')
const pct = (part: number, whole: number) => (whole > 0 ? Math.round((100 * part) / whole) : 0)

/** Showdown win rates only count once a player has been to a few. */
const MIN_SHOWDOWNS = 3

interface Award {
  icon: string
  title: string
  who: string
  detail: string
}

/**
 * The game in numbers, under the final standings: a few awards, then each
 * player's line. Everything here was public at the table.
 */
export function GameStats({ state }: { state: State }) {
  const stats = state.stats
  if (!stats || stats.hands === 0) return null
  const name = (id: string) => (id === state.you ? 'You' : (state.players.find((p) => p.id === id)?.name ?? '?'))
  const top = stats.variants[0]
  const awards = gameAwards(stats, state, name)

  return (
    <section className="stats" aria-labelledby="stats-title">
      <h2 id="stats-title">Game stats</h2>
      <p className="stats-summary muted">
        {fmt(stats.hands)} hand{stats.hands === 1 ? '' : 's'}
        {top && stats.variants.length > 1 && ` · most played: ${variantName(top.variant)} (${top.hands})`}
        {top && stats.variants.length === 1 && ` of ${variantName(top.variant)}`}
      </p>

      {awards.length > 0 && (
        <ul className="stats-awards">
          {awards.map((a) => (
            <li key={a.title} className="stats-award">
              <span className="stats-award-icon" aria-hidden>
                {a.icon}
              </span>
              <span className="stats-award-title">{a.title}</span>
              <strong className="stats-award-who">{a.who}</strong>
              <span className="stats-award-detail">{a.detail}</span>
            </li>
          ))}
        </ul>
      )}

      <ul className="stats-players">
        {stats.players.map((p) => (
          <li key={p.id} className="stats-player" aria-label={`Stats for ${name(p.id)}`}>
            <strong>{name(p.id)}</strong>
            <span>
              Won {p.won} of {p.hands} · folded {pct(p.folded, p.hands)}%
              {p.showdowns > 0 && ` · showdowns ${p.showdownsWon}/${p.showdowns}`}
            </span>
            <span className="muted">{variantLine(p)}</span>
          </li>
        ))}
      </ul>
    </section>
  )
}

/** "Omaha +3,400 · Hold'em −1,200": best and worst game, or the only one. */
function variantLine(p: PlayerStats): string {
  const played = p.net ?? []
  if (played.length === 0) return ''
  if (played.length === 1) return `${variantName(played[0].variant)} ${signed(played[0].chips)}`
  const sorted = [...played].sort((a, b) => b.chips - a.chips)
  const best = sorted[0]
  const worst = sorted[sorted.length - 1]
  return `Best: ${variantName(best.variant)} ${signed(best.chips)} · Worst: ${variantName(worst.variant)} ${signed(worst.chips)}`
}

/** Everyone tied for the top value of score (ignoring players who score null). */
function leaders<T extends { id: string }>(players: T[], score: (p: T) => number | null): { ids: string[]; value: number } | null {
  let best: number | null = null
  let ids: string[] = []
  for (const p of players) {
    const s = score(p)
    if (s === null) continue
    if (best === null || s > best) {
      best = s
      ids = [p.id]
    } else if (s === best) {
      ids.push(p.id)
    }
  }
  return best === null ? null : { ids, value: best }
}

function gameAwards(stats: Stats, state: State, name: (id: string) => string): Award[] {
  const names = (ids: string[]) => ids.map(name).join(' & ')
  const out: Award[] = []

  const pot = stats.biggestPot
  if (pot) {
    out.push({
      icon: '💰',
      title: 'Biggest pot',
      who: fmt(pot.chips),
      detail: `Hand #${pot.hand}, ${variantName(pot.variant)}, to ${names(pot.winners)}`,
    })
  }

  const wins = leaders(stats.players, (p) => (p.won > 0 ? p.won : null))
  if (wins) {
    const p = stats.players.find((q) => q.id === wins.ids[0])!
    out.push({
      icon: '🎯',
      title: 'Most hands won',
      who: names(wins.ids),
      detail: wins.ids.length === 1 ? `${wins.value} of ${p.hands}` : `${wins.value} each`,
    })
  }

  const showdown = leaders(stats.players, (p) => (p.showdowns >= MIN_SHOWDOWNS ? pct(p.showdownsWon, p.showdowns) : null))
  if (showdown && showdown.value > 0) {
    out.push({
      icon: '🃏',
      title: 'Showdown king',
      who: names(showdown.ids),
      detail: `won ${showdown.value}% of showdowns`,
    })
  }

  const rock = leaders(stats.players, (p) => (p.hands >= 5 && p.folded > 0 ? pct(p.folded, p.hands) : null))
  if (rock) {
    out.push({ icon: '🧱', title: 'The rock', who: names(rock.ids), detail: `folded ${rock.value}% of hands` })
  }

  const borrower = leaders(state.players, (p) => (p.loans > 0 ? p.loans : null))
  if (borrower) {
    out.push({
      icon: '🏦',
      title: "Bank's best customer",
      who: names(borrower.ids),
      detail: `${borrower.value} loan${borrower.value === 1 ? '' : 's'}`,
    })
  }
  return out
}
