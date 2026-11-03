import type { ReactNode } from 'react'
import type { Action, ClientMsg, PlayerView, State } from '../protocol/types.ts'
import { variantName } from '../variants.ts'
import { ActionBar } from './ActionBar.tsx'
import { Card, CardBack } from './Card.tsx'
import { Countdown } from './Countdown.tsx'
import { ResultBanner } from './ResultBanner.tsx'
import { ShowPicker } from './ShowPicker.tsx'
import { winningCards } from './winning.ts'

interface Props {
  state: State
  clockOffset: number
  onAct: (a: Action) => void
  send: (m: ClientMsg) => void
}

export function Table({ state, clockOffset, onAct, send }: Props) {
  const me = state.players.find((p) => p.id === state.you)
  const hand = state.hand
  const isHost = state.you === state.hostId
  const name = (id?: string) => state.players.find((p) => p.id === id)?.name ?? '?'
  const winning = winningCards(hand)

  return (
    <div className="table-layout">
      <main className="table">
        <TableHeader state={state} clockOffset={clockOffset} name={name} />

        {hand && (
          <div className="board" aria-label="Board">
            {hand.boards.map((board, b) => (
              <div className="board-row" key={b}>
                {board.map((c) => (
                  <Card key={c} code={c} highlight={highlightOf(c, winning)} />
                ))}
                {Array.from({ length: 5 - board.length }, (_, i) => (
                  <span className="card card-empty" key={`e${i}`} />
                ))}
              </div>
            ))}
            <div className="pot">Pot {hand.pot}</div>
          </div>
        )}

        {hand && <ResultBanner state={state} hand={hand} clockOffset={clockOffset} />}

        <ul className="seats">
          {state.players.map((p) => (
            <Seat key={p.id} p={p} state={state} clockOffset={clockOffset} winning={winning} />
          ))}
        </ul>

        {state.choosing && state.choosing.dealerId === state.you && (
          <div className="panel choose">
            <h3>You're the dealer — pick the game</h3>
            <div className="choose-options">
              {state.choosing.options.map((v) => (
                <button key={v} className="btn" onClick={() => send({ type: 'chooseVariant', variant: v })}>
                  {variantName(v)}
                </button>
              ))}
            </div>
          </div>
        )}

        {me?.inHand && me.hole && !hand?.done && (
          // Pinned to the bottom of the screen so your cards and actions are
          // always in reach on a phone.
          <div className={`dock${hand?.legal ? ' dock-active' : ''}`}>
            {!hand?.legal?.show && (
              <div className="dock-cards" aria-label="Your cards">
                {me.hole.map((c) => (
                  <span key={c} className={me.shown?.includes(c) ? 'shown-card' : undefined}>
                    <Card code={c} />
                  </span>
                ))}
                {hand?.legal ? (
                  <span className="dock-turn">Your turn</span>
                ) : (
                  me.folded && <span className="muted">Folded</span>
                )}
              </div>
            )}
            {hand?.legal?.show && <ShowPicker hole={me.hole} count={hand.reveal ?? 2} onAct={onAct} />}
            {hand?.legal && !hand.legal.show && (
              <ActionBar
                key={`${hand.number}-${hand.street}-${hand.currentBet}`}
                legal={hand.legal}
                bet={me.bet}
                mult={me.mult}
                pot={hand.pot}
                onAct={onAct}
              />
            )}
          </div>
        )}

        {me?.broke && state.phase === 'playing' && (
          <div className="panel loan">
            <p>
              You're out of chips. Borrow {state.settings.loanAmount}
              {me.loans < state.settings.penalizedLoans
                ? ` (adds ${state.settings.loanAmount + state.settings.loanPenalty} to your debt, including the ${state.settings.loanPenalty} penalty)`
                : ' (no penalty)'}
              ?
            </p>
            <button className="btn btn-primary" onClick={() => send({ type: 'takeLoan' })}>
              Take loan
            </button>
          </div>
        )}

        <div className="table-controls">
          {me &&
            state.phase === 'playing' &&
            (me.sittingOut ? (
              <button className="btn btn-small" onClick={() => send({ type: 'sitIn' })}>
                Sit back in
              </button>
            ) : (
              <button className="btn btn-small" onClick={() => send({ type: 'sitOut' })}>
                Sit out next hand
              </button>
            ))}
          {isHost && state.phase === 'playing' && (
            <button
              className="btn btn-small btn-link"
              onClick={() => {
                if (window.confirm('End the game for everyone after this hand?')) send({ type: 'endGame' })
              }}
            >
              End game
            </button>
          )}
        </div>
      </main>

      <aside className="sidebar">
        <section className="panel">
          <h3>Leaderboard</h3>
          <ol className="leaderboard">
            {state.leaderboard.map((s) => (
              <li key={s.id}>
                <span>{s.name}</span>
                <span className={s.profit >= 0 ? 'profit-up' : 'profit-down'}>
                  {s.profit > 0 ? '+' : ''}
                  {s.profit}
                </span>
              </li>
            ))}
          </ol>
        </section>
        <details className="panel">
          <summary>
            <h3>Hand log</h3>
          </summary>
          <ol className="log">
            {state.log.map((line, i) => (
              <li key={i}>{line}</li>
            ))}
          </ol>
        </details>
      </aside>
    </div>
  )
}

function TableHeader({
  state,
  clockOffset,
  name,
}: {
  state: State
  clockOffset: number
  name: (id?: string) => string
}) {
  const hand = state.hand
  let status: ReactNode = null
  if (state.phase === 'ended') {
    status = 'Game over — final standings on the right.'
  } else if (state.stage === 'waiting') {
    status = 'Waiting for at least two players with chips…'
  } else if (state.choosing) {
    status = (
      <>
        {name(state.choosing.dealerId)} is picking the game{' '}
        <Countdown deadline={state.choosing.deadline} clockOffset={clockOffset} />
      </>
    )
  } else if (hand && !hand.done && hand.toAct) {
    status = (
      <>
        {hand.showing
          ? hand.toAct === state.you
            ? 'Pick cards to show'
            : `${name(hand.toAct)} is picking cards to show`
          : hand.toAct === state.you
            ? 'Your turn'
            : `${name(hand.toAct)} to act`}{' '}
        {hand.deadline ? <Countdown deadline={hand.deadline} clockOffset={clockOffset} /> : null}
      </>
    )
  } else if (hand?.done) {
    status = 'Hand over'
  }
  return (
    <header className="table-header">
      {hand && (
        <span>
          Hand #{hand.number} · {variantName(hand.variant)} · {hand.street}
        </span>
      )}
      <span className="status" role="status">
        {status}
      </span>
    </header>
  )
}

function Seat({
  p,
  state,
  clockOffset,
  winning,
}: {
  p: PlayerView
  state: State
  clockOffset: number
  winning: Set<string>
}) {
  const hand = state.hand
  const acting = hand && !hand.done && hand.toAct === p.id
  const holeCount = p.holeCount ?? 2
  const shown = p.shown ?? []
  const classes = [
    'seat',
    acting && 'seat-acting',
    p.folded && 'seat-folded',
    p.id === state.you && 'seat-me',
    (p.won ?? 0) > 0 && hand?.done && 'seat-won',
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <li className={classes} aria-label={`Seat ${p.name}`}>
      <div className="seat-name">
        {p.name}
        {p.dealer && <span className="tag tag-dealer" title="Dealer">D</span>}
        {p.id === state.hostId && <span className="tag">host</span>}
        {!p.connected && <span className="tag tag-muted">offline</span>}
      </div>
      <div className="seat-stack">{p.stack} chips</div>
      {p.inHand && (
        <div className="seat-cards">
          {p.hole
            ? p.hole.map((c) => <Card key={c} code={c} small highlight={highlightOf(c, winning)} />)
            : !p.folded && (
                <>
                  {shown.map((c) => (
                    <Card key={c} code={c} small />
                  ))}
                  {Array.from({ length: holeCount - shown.length }, (_, i) => (
                    <CardBack key={i} small />
                  ))}
                </>
              )}
        </div>
      )}
      <div className="seat-status">
        {acting && hand?.deadline ? <Countdown deadline={hand.deadline} clockOffset={clockOffset} /> : null}
        {p.folded && 'Folded'}
        {p.allIn && !hand?.done && 'All-in'}
        {p.bet > 0 && !hand?.done && `Bet ${p.bet * p.mult}`}
        {p.handDesc && <span className="hand-desc">{p.handDesc}</span>}
        {(p.won ?? 0) > 0 && hand?.done && <span className="won">Won {p.won}</span>}
        {p.sittingOut && 'Sitting out'}
        {p.broke && 'Broke'}
      </div>
      {(p.loans > 0 || p.debt > 0) && (
        <div className="seat-debt">
          Debt {p.debt} ({p.loans} loan{p.loans === 1 ? '' : 's'})
        </div>
      )}
    </li>
  )
}

/** After a showdown, light up cards in a winning hand and dim the rest. */
function highlightOf(card: string, winning: Set<string>): 'win' | 'dim' | undefined {
  if (winning.size === 0) return undefined
  return winning.has(card) ? 'win' : 'dim'
}
