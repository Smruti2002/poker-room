import type { HandView, State } from '../protocol/types.ts'
import { Card } from './Card.tsx'
import { Countdown } from './Countdown.tsx'

/** Who won the last hand and with what, shown until the next hand starts. */
export function ResultBanner({ state, hand, clockOffset }: { state: State; hand: HandView; clockOffset: number }) {
  const winners = hand.winners ?? []
  if (!hand.done || winners.length === 0) return null
  const name = (id: string) => (id === state.you ? 'You' : (state.players.find((p) => p.id === id)?.name ?? '?'))
  const split = winners.length > 1

  return (
    <section className="result" role="status" aria-live="polite" aria-label="Hand result">
      {winners.map((w) => (
        <div className="result-row" key={w.playerId}>
          <p className="result-line">
            <strong>{name(w.playerId)}</strong> {w.playerId === state.you ? 'win' : 'wins'}{' '}
            <strong className="result-chips">{w.chips}</strong>
            {split && ' (split pot)'}
            {w.desc ? (
              <>
                {' '}
                with <strong>{w.desc}</strong>
              </>
            ) : (
              ' — everyone else folded'
            )}
          </p>
          {w.cards?.map((five, b) => (
            <div className="result-cards" key={b} aria-label="Winning hand">
              {five.map((c) => (
                <Card key={c} code={c} small highlight="win" />
              ))}
            </div>
          ))}
        </div>
      ))}
      {hand.deadline && state.stage === 'result' ? (
        <p className="muted small result-next">
          Next hand in <Countdown deadline={hand.deadline} clockOffset={clockOffset} />
        </p>
      ) : null}
    </section>
  )
}
