const SUITS: Record<string, { symbol: string; name: string; red: boolean }> = {
  s: { symbol: '♠', name: 'spades', red: false },
  h: { symbol: '♥', name: 'hearts', red: true },
  d: { symbol: '♦', name: 'diamonds', red: true },
  c: { symbol: '♣', name: 'clubs', red: false },
}

const RANK_NAMES: Record<string, string> = {
  A: 'Ace', K: 'King', Q: 'Queen', J: 'Jack', T: 'Ten',
  '9': 'Nine', '8': 'Eight', '7': 'Seven', '6': 'Six', '5': 'Five', '4': 'Four', '3': 'Three', '2': 'Two',
}

/** A face-up card from its two-character code, e.g. "As" or "Td". */
export function Card({ code, small = false }: { code: string; small?: boolean }) {
  const rank = code[0]
  const suit = SUITS[code[1]]
  if (!suit || !RANK_NAMES[rank]) return <span className="card card-bad">?</span>
  return (
    <span
      className={`card${suit.red ? ' card-red' : ''}${small ? ' card-small' : ''}`}
      aria-label={`${RANK_NAMES[rank]} of ${suit.name}`}
      title={`${RANK_NAMES[rank]} of ${suit.name}`}
    >
      {rank === 'T' ? '10' : rank}
      {suit.symbol}
    </span>
  )
}

/** A face-down card. */
export function CardBack({ small = false }: { small?: boolean }) {
  return <span className={`card card-back${small ? ' card-small' : ''}`} aria-label="Hidden card" />
}
