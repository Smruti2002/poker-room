import type { HandView, State, WinnerView } from '../protocol/types.ts'
import { Card } from './Card.tsx'

/** Who won the last hand and with what, shown until the next hand starts. */
export function ResultBanner({ state, hand, onNext }: { state: State; hand: HandView; onNext: () => void }) {
  const winners = hand.winners ?? []
  if (!hand.done || winners.length === 0) return null
  const name = (id: string) => (id === state.you ? 'You' : (state.players.find((p) => p.id === id)?.name ?? '?'))
  const split = winners.length > 1

  return (
    <section className="result" role="status" aria-live="polite" aria-label="Hand result">
      {winners.map((w) => {
        const wins = boardsWon(hand, w)
        return (
          <div className="result-row" key={w.playerId}>
            <p className="result-line">
              <strong>{name(w.playerId)}</strong> {w.playerId === state.you ? 'win' : 'wins'}{' '}
              <strong className="result-chips">{w.chips}</strong>
              {split && ' (split pot)'}
              {!w.desc && ' — everyone else folded'}
              {w.desc && wins.length === 1 && wins[0].board < 0 && (
                <>
                  {' '}
                  with <strong>{wins[0].desc}</strong>
                </>
              )}
            </p>
            {wins.map(({ board, desc, cards }) => (
              <div className="result-board" key={board}>
                {board >= 0 && (
                  <p className="result-board-line">
                    Board {board + 1}: <strong>{desc}</strong>
                  </p>
                )}
                <div className="result-cards" aria-label="Winning hand">
                  {cards.map((c) => (
                    <Card key={c} code={c} small highlight="win" />
                  ))}
                </div>
              </div>
            ))}
          </div>
        )
      })}
      {state.stage === 'result' && (
        <button className="btn btn-primary result-next" onClick={onNext}>
          Next hand
        </button>
      )}
    </section>
  )
}

/**
 * The hands a winner should be credited with. With one board that's their
 * hand (board -1); with several, only the boards they won or tied.
 */
function boardsWon(hand: HandView, w: WinnerView): { board: number; desc: string; cards: string[] }[] {
  const descs = (w.desc ?? '').split(' | ')
  const cards = w.cards ?? []
  if (hand.boards.length === 1) return cards.length ? [{ board: -1, desc: descs[0], cards: cards[0] }] : []
  return cards
    .map((five, b) => ({ board: b, desc: descs[b] ?? '', cards: five }))
    .filter(({ board }) => hand.boardWinners?.[board]?.includes(w.playerId))
}
