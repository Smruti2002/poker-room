import type { ReactNode } from 'react'
import type { Action, ClientMsg, PlayerView, State } from '../protocol/types.ts'
import { variantName } from '../variants.ts'
import { ActionBar } from './ActionBar.tsx'
import { KickButton } from './KickVotes.tsx'
import { FinalStandings } from './FinalStandings.tsx'
import { RaiseBanner, SeatCallout } from './BetCallout.tsx'
import { RoundSteps } from './RoundSteps.tsx'
import { HandCards } from './HandCards.tsx'
import { useCardOrder } from '../cardOrder.ts'
import { Card, CardBack } from './Card.tsx'
import { useNow } from './useNow.ts'
import { ResultBanner } from './ResultBanner.tsx'
import { ChoosePicker } from './ChoosePicker.tsx'
import { ShowPicker } from './ShowPicker.tsx'
import { SplitPicker } from './SplitPicker.tsx'
import { winningCards } from './winning.ts'

/** How long a reaction floats over a seat. */
const REACTION_MS = 3500

interface Props {
  state: State
  clockOffset: number
  onAct: (a: Action) => void
  send: (m: ClientMsg) => void
}

export function Table({ state, clockOffset, onAct, send }: Props) {
  const me = state.players.find((p) => p.id === state.you)
  const order = useCardOrder(state.code, state.hand?.number ?? 0, me?.hole ?? [])
  const hand = state.hand
  const isHost = state.you === state.hostId
  const name = (id?: string) => state.players.find((p) => p.id === id)?.name ?? '?'
  const winning = winningCards(hand)
  // Re-render while reactions are showing so they disappear on time.
  const now = useNow(state.reactions.length > 0 ? 500 : 0) + clockOffset
  const reactionOf = (id: string) =>
    state.reactions.findLast((r) => r.playerId === id && now - r.at < REACTION_MS)?.emoji

  // Once the game is over, the final standings take the table's place.
  if (state.phase === 'ended') {
    return (
      <div className="table-layout table-layout-ended">
        <main className="table">
          <header className="table-header">
            <span className="status" role="status">
              Game over
            </span>
          </header>
          <FinalStandings state={state} send={send} />
        </main>
      </div>
    )
  }

  return (
    <div className="table-layout">
      <main className="table">
        <TableHeader state={state} name={name} />
        <RaiseBanner state={state} />

        {hand && <RoundSteps hand={hand} />}
        {hand && (
          <div className="board" aria-label="Board">
            {hand.boards.map((board, b) => (
              <div className="board-row" key={b}>
                {hand.boards.length > 1 && (
                  <span className="board-label">
                    {b + 1}
                    {hand.boardWinners?.[b] && (
                      <span className="board-winner">{hand.boardWinners[b].map((id) => name(id)).join(' & ')}</span>
                    )}
                  </span>
                )}
                {board.map((c) => (
                  <Card key={c} code={c} highlight={highlightOf(c, winning)} />
                ))}
                {Array.from({ length: 5 - board.length }, (_, i) => (
                  <span className="card card-empty" key={`e${i}`} />
                ))}
              </div>
            ))}
            <div className="pot" key={hand.pot}>
              Pot {hand.pot}
            </div>
          </div>
        )}

        {hand && <ResultBanner state={state} hand={hand} onNext={() => send({ type: 'nextHand' })} />}

        <ul className="seats">
          {state.players.filter((p) => !p.kicked || (p.inHand && hand && !hand.done)).map((p) => (
            <Seat
              key={p.id}
              p={p}
              hole={p.id === state.you && p.hole ? order.cards : p.hole}
              state={state}
              winning={winning}
              reaction={reactionOf(p.id)}
              send={send}
            />
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

        {me?.inHand && (me.hole || hand?.legal) && !hand?.done && (
          // Pinned to the bottom of the screen so your cards and actions are
          // always in reach on a phone.
          <div className={`dock${hand?.legal ? ' dock-active' : ''}`}>
            {me.hole && !hand?.legal?.show && !hand?.legal?.allocate && (
              <div className="dock-cards">
                <HandCards
                  cards={order.cards}
                  shown={me.shown}
                  nextSort={order.nextSort}
                  onMove={order.move}
                  onSwap={order.swap}
                  onSort={order.sort}
                />
                {hand?.legal ? (
                  <span className="dock-turn">Your turn</span>
                ) : me.allocated ? (
                  <span className="muted">Locked in, waiting for the others</span>
                ) : (
                  me.folded && <span className="muted">Folded</span>
                )}
              </div>
            )}
            {hand?.hint && !hand.legal?.allocate && (
              <p className="hint">
                You have: <strong>{hand.hint}</strong>
              </p>
            )}
            {me.mult > 1 && !me.folded && (
              <p className="double-note">You took 6 cards: every bet costs you double.</p>
            )}
            {hand?.legal?.choose && <ChoosePicker max={hand.legal.chooseMax ?? 4} onAct={onAct} />}
            {hand?.legal?.show && me.hole && <ShowPicker hole={order.cards} count={hand.reveal ?? 2} onAct={onAct} />}
            {hand?.legal?.allocate && me.hole && <SplitPicker hole={order.cards} boards={hand.boards} onAct={onAct} />}
            {hand?.legal && !hand.legal.show && !hand.legal.choose && !hand.legal.allocate && (
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
                <span>
                  {s.name}
                  {s.kicked && <span className="tag tag-muted">removed</span>}
                </span>
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
            {(state.log ?? []).map((line, i) => (
              <li key={i}>{line}</li>
            ))}
          </ol>
        </details>
      </aside>
    </div>
  )
}

function TableHeader({ state, name }: { state: State; name: (id?: string) => string }) {
  const hand = state.hand
  let status: ReactNode = null
  if (state.phase === 'ended') {
    status = 'Game over'
  } else if (state.stage === 'waiting') {
    status = 'Waiting for at least two players with chips…'
  } else if (state.choosing) {
    status = (
      <>
        {name(state.choosing.dealerId)} is picking the game
      </>
    )
  } else if (hand?.allocating) {
    // Everyone splits at once: say who we're still waiting for.
    const waiting = state.players.filter((p) => p.inHand && !p.folded && !p.allocated && p.id !== state.you)
    status = (
      <>
        {hand.legal?.allocate
          ? 'Split your cards across the boards'
          : `Waiting for ${waiting.map((p) => p.name).join(', ')} to split`}
      </>
    )
  } else if (hand && !hand.done && hand.toAct) {
    status = (
      <>
        {hand.picking
          ? hand.toAct === state.you
            ? 'Choose 4 or 6 cards'
            : `${name(hand.toAct)} is choosing 4 or 6 cards`
          : hand.showing
          ? hand.toAct === state.you
            ? 'Pick cards to show'
            : `${name(hand.toAct)} is picking cards to show`
          : hand.toAct === state.you
            ? 'Your turn'
            : `${name(hand.toAct)} to act`}
      </>
    )
  } else if (hand?.done) {
    status = 'Hand over'
  }
  return (
    <header className="table-header">
      {hand && (
        <span>
          Hand #{hand.number} · {variantName(hand.variant)}
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
  hole,
  state,
  winning,
  reaction,
  send,
}: {
  p: PlayerView
  hole?: string[] // the player's cards, in your own order for your seat
  state: State
  winning: Set<string>
  reaction?: string
  send: (m: ClientMsg) => void
}) {
  const hand = state.hand
  const acting = hand && !hand.done && hand.toAct === p.id
  // Who the table is waiting on: the dealer picking the game, the player to
  // act, or anyone still splitting their cards. There are no timers, so the
  // host can skip them.
  const waiting =
    state.choosing?.dealerId === p.id ||
    acting ||
    (hand?.allocating && p.inHand && !p.folded && !p.allocated)
  const canSkip = waiting && state.you === state.hostId && p.id !== state.you
  const skip = () => {
    if (window.confirm(`Skip ${p.name}? They'll get the default move: check if possible, otherwise fold (or the default pick or split).`)) {
      send({ type: 'skip', target: p.id })
    }
  }
  const holeCount = p.holeCount ?? 0 // 0 before the deal (Dacoit choices)
  const shown = p.shown ?? []
  const classes = [
    'seat',
    waiting && 'seat-acting',
    p.folded && 'seat-folded',
    p.id === state.you && 'seat-me',
    (p.won ?? 0) > 0 && hand?.done && 'seat-won',
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <li className={classes} aria-label={`Seat ${p.name}`}>
      {reaction && (
        <span className="reaction" key={reaction} aria-label={`${p.name} reacts ${reaction}`}>
          {reaction}
        </span>
      )}
      <div className="seat-name">
        {p.name}
        {p.dealer && <span className="tag tag-dealer" title="Dealer">D</span>}
        {(p.picked ?? 0) > 0 && p.inHand && (
          <span className={`tag${p.mult > 1 ? ' tag-double' : ''}`} title={p.mult > 1 ? 'Pays double for every bet' : undefined}>
            {p.picked} cards{p.mult > 1 ? ' · 2×' : ''}
          </span>
        )}
        {p.allocated && <span className="tag tag-locked">locked in</span>}
        {p.id === state.hostId && <span className="tag">host</span>}
        {p.kicked ? <span className="tag tag-muted">removed</span> : !p.connected && <span className="tag tag-muted">offline</span>}
        <KickButton p={p} state={state} send={send} />
      </div>
      <div className="seat-stack">{p.stack} chips</div>
      {p.inHand && (
        <div className="seat-cards">
          {hole
            ? shown.length > 0
              ? // Show 2 Play 2: the cards shown before betting, then the hidden ones.
                [hole.filter((c) => shown.includes(c)), hole.filter((c) => !shown.includes(c))].map((group, g) => (
                  <span key={g} className="seat-card-group" aria-label={g === 0 ? 'Shown' : 'Hidden'}>
                    {group.map((c) => (
                      <Card key={c} code={c} small highlight={highlightOf(c, winning)} />
                    ))}
                  </span>
                ))
              : hole.map((c) => <Card key={c} code={c} small highlight={highlightOf(c, winning)} />)
            : !p.folded &&
              (shown.length > 0 ? (
                <>
                  <span className="seat-card-group" aria-label="Shown">
                    {shown.map((c) => (
                      <Card key={c} code={c} small />
                    ))}
                  </span>
                  <span className="seat-card-group" aria-label="Hidden">
                    {Array.from({ length: holeCount - shown.length }, (_, i) => (
                      <CardBack key={i} small />
                    ))}
                  </span>
                </>
              ) : (
                Array.from({ length: holeCount }, (_, i) => <CardBack key={i} small />)
              ))}
        </div>
      )}
      <div className="seat-status">
        {waiting && (
          <span className="thinking" aria-label={`Waiting for ${p.name}`}>
            <span />
            <span />
            <span />
          </span>
        )}
        {canSkip && (
          <button className="btn btn-small btn-skip" onClick={skip}>
            Skip
          </button>
        )}
        {p.folded && 'Folded'}
        {p.allIn && !hand?.done && 'All-in'}
        {p.lastBet ? (
          <SeatCallout c={p.lastBet} />
        ) : (
          p.bet > 0 &&
          !hand?.done && (
            <span className="bet-chip" key={p.bet}>
              Bet {p.bet * p.mult}
            </span>
          )
        )}
        {p.handDesc && <HandDesc desc={p.handDesc} />}
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

/** A hand description; multi-board hands are listed per board. */
function HandDesc({ desc }: { desc: string }) {
  const parts = desc.split(' | ')
  if (parts.length === 1) return <span className="hand-desc">{desc}</span>
  return (
    <span className="hand-desc hand-desc-boards">
      {parts.map((d, b) => (
        <span key={b}>
          <span className="board-num">{b + 1}</span> {d}
        </span>
      ))}
    </span>
  )
}
