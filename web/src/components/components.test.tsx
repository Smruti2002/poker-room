import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Legal } from '../protocol/types.ts'
import { player, settings, state } from '../test/fixtures.ts'
import { ActionBar } from './ActionBar.tsx'
import { Card } from './Card.tsx'
import { HandCards } from './HandCards.tsx'
import { KickVotes } from './KickVotes.tsx'
import { Lobby } from './Lobby.tsx'
import { Table } from './Table.tsx'

describe('Card', () => {
  it('renders rank and suit, red for hearts and diamonds', () => {
    render(<Card code="Td" />)
    const card = screen.getByLabelText('Ten of diamonds')
    expect(card).toHaveTextContent('10♦')
    expect(card).toHaveClass('card-red')
  })

  it('renders black suits and rejects bad codes', () => {
    render(
      <>
        <Card code="As" />
        <Card code="Zz" />
      </>,
    )
    expect(screen.getByLabelText('Ace of spades')).not.toHaveClass('card-red')
    expect(screen.getByText('?')).toBeInTheDocument()
  })
})

describe('ActionBar', () => {
  const facingBet: Legal = { fold: true, check: false, call: true, callUnits: 20, bet: false, raise: true, minTo: 40, maxTo: 1000 }

  it('offers fold, call and raise when facing a bet', () => {
    const onAct = vi.fn()
    render(<ActionBar legal={facingBet} bet={0} mult={1} pot={30} onAct={onAct} />)
    fireEvent.click(screen.getByRole('button', { name: 'Fold' }))
    fireEvent.click(screen.getByRole('button', { name: 'Call 20' }))
    fireEvent.click(screen.getByRole('button', { name: 'Raise to 40' }))
    expect(onAct.mock.calls).toEqual([[{ type: 'fold' }], [{ type: 'call' }], [{ type: 'raise', to: 40 }]])
    expect(screen.queryByRole('button', { name: 'Check' })).toBeNull()
  })

  it('pot preset is a pot-sized raise', () => {
    const onAct = vi.fn()
    render(<ActionBar legal={facingBet} bet={0} mult={1} pot={30} onAct={onAct} />)
    // Call 20 to make the pot 50, then raise by 50: to 70.
    fireEvent.click(screen.getByRole('button', { name: 'Pot' }))
    fireEvent.click(screen.getByRole('button', { name: 'Raise to 70' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'raise', to: 70 })
  })

  it('the amount box can be cleared and retyped freely', () => {
    const onAct = vi.fn()
    render(<ActionBar legal={facingBet} bet={0} mult={1} pot={30} onAct={onAct} />)
    const box = screen.getByLabelText('raise to')

    fireEvent.change(box, { target: { value: '' } })
    expect(box).toHaveValue('') // stays empty instead of snapping back
    expect(screen.getByRole('button', { name: 'Raise' })).toBeDisabled()

    fireEvent.change(box, { target: { value: '1' } })
    expect(box).toHaveValue('1')
    expect(screen.getByRole('note')).toHaveTextContent('Enter 40–1000')
    fireEvent.change(box, { target: { value: '15' } })
    fireEvent.change(box, { target: { value: '150' } })
    fireEvent.click(screen.getByRole('button', { name: 'Raise to 150' }))

    // Letters are ignored; Enter raises.
    fireEvent.change(box, { target: { value: '2x00' } })
    expect(box).toHaveValue('200')
    fireEvent.keyDown(box, { key: 'Enter' })
    expect(onAct.mock.calls).toEqual([[{ type: 'raise', to: 150 }], [{ type: 'raise', to: 200 }]])
  })

  it('out-of-range amounts disable raising and snap into range on blur', () => {
    const onAct = vi.fn()
    render(<ActionBar legal={facingBet} bet={0} mult={1} pot={30} onAct={onAct} />)
    const box = screen.getByLabelText('raise to')
    fireEvent.change(box, { target: { value: '99999' } })
    expect(screen.getByRole('button', { name: 'Raise' })).toBeDisabled()
    fireEvent.blur(box)
    expect(box).toHaveValue('1000')
    fireEvent.click(screen.getByRole('button', { name: 'Raise to 1000' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'raise', to: 1000 })
  })

  it('offers check and bet when nobody has bet', () => {
    const onAct = vi.fn()
    const legal: Legal = { fold: false, check: true, call: false, callUnits: 0, bet: true, raise: false, minTo: 20, maxTo: 500 }
    render(<ActionBar legal={legal} bet={0} mult={1} pot={60} onAct={onAct} />)
    fireEvent.click(screen.getByRole('button', { name: 'Check' }))
    fireEvent.click(screen.getByRole('button', { name: 'Bet 20' }))
    expect(onAct.mock.calls).toEqual([[{ type: 'check' }], [{ type: 'bet', to: 20 }]])
    expect(screen.queryByRole('button', { name: 'Fold' })).toBeNull()
  })

  it('shows a single all-in button when that is the only raise, in chips for double-pay players', () => {
    const legal: Legal = { ...facingBet, minTo: 30, maxTo: 30 }
    render(<ActionBar legal={legal} bet={0} mult={2} pot={30} onAct={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'All-in 60' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Call 40' })).toBeInTheDocument()
    expect(screen.queryByRole('slider')).toBeNull()
  })
})

describe('Lobby', () => {
  it('lets the host save settings and start once two players are in', () => {
    const onSave = vi.fn()
    const onStart = vi.fn()
    render(<Lobby state={state()} onSave={onSave} onStart={onStart} send={vi.fn()} />)

    const start = screen.getByRole('button', { name: 'Start game' })
    expect(start).toBeEnabled()

    fireEvent.change(screen.getByLabelText('Starting stack'), { target: { value: '5000' } })
    expect(start).toBeDisabled() // unsaved changes
    fireEvent.click(screen.getByRole('button', { name: 'Save settings' }))
    expect(onSave).toHaveBeenCalledWith({ ...settings, startingStack: 5000 })
  })

  it('needs two players to start', () => {
    render(<Lobby state={state({ players: [player({ id: 'p1', name: 'Ann' })] })} onSave={vi.fn()} onStart={vi.fn()} send={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Start game' })).toBeDisabled()
  })

  it('shows read-only live settings to other players', () => {
    const { rerender } = render(<Lobby state={state({ you: 'p2' })} onSave={vi.fn()} onStart={vi.fn()} send={vi.fn()} />)
    expect(screen.getByText('Waiting for the host to start…')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Start game' })).toBeNull()
    expect(screen.getByLabelText('Starting stack')).toBeDisabled()

    rerender(<Lobby state={state({ you: 'p2', settings: { ...settings, startingStack: 7000 } })} onSave={vi.fn()} onStart={vi.fn()} send={vi.fn()} />)
    expect(screen.getByLabelText('Starting stack')).toHaveValue(7000)
  })
})

describe('Table', () => {
  const inHand = state({
    phase: 'playing',
    stage: 'hand',
    players: [
      player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 2, hole: ['As', 'Kd'], bet: 10 }),
      player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 2, bet: 20, dealer: true }),
      player({ id: 'p3', name: 'Cy', inHand: true, holeCount: 2, folded: true }),
    ],
    hand: {
      number: 3,
      variant: 'holdem',
      street: 'Pre-flop',
      boards: [[]],
      pot: 30,
      toAct: 'p1',
      currentBet: 20,
      legal: { fold: true, check: false, call: true, callUnits: 10, bet: false, raise: true, minTo: 40, maxTo: 1990 },
      done: false,
    },
  })

  it('shows my cards, hides other live hands, and offers actions on my turn', () => {
    const onAct = vi.fn()
    render(<Table state={inHand} clockOffset={0} onAct={onAct} send={vi.fn()} />)

    const ann = screen.getByLabelText('Seat Ann')
    expect(within(ann).getByLabelText('Ace of spades')).toBeInTheDocument()
    expect(within(screen.getByLabelText('Seat Bob')).getAllByLabelText('Hidden card')).toHaveLength(2)
    expect(within(screen.getByLabelText('Seat Cy')).queryAllByLabelText('Hidden card')).toHaveLength(0)
    expect(screen.getByRole('status')).toHaveTextContent('Your turn')
    expect(screen.getByText(/Hand #3 · Texas Hold'em/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Call 10' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'call' })
  })

  it('offers a loan to a broke player, with the penalty spelled out', () => {
    const send = vi.fn()
    const s = state({
      phase: 'playing',
      stage: 'waiting',
      players: [player({ id: 'p1', name: 'Ann', stack: 0, broke: true }), player({ id: 'p2', name: 'Bob' })],
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={send} />)
    expect(screen.getByText(/adds 3000 to your debt/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Take loan' }))
    expect(send).toHaveBeenCalledWith({ type: 'takeLoan' })
  })

  it('lets the dealer pick the variant', () => {
    const send = vi.fn()
    const s = state({
      phase: 'playing',
      stage: 'choosing',
      choosing: { dealerId: 'p1', options: ['holdem', 'omaha'] },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={send} />)
    fireEvent.click(screen.getByRole('button', { name: 'Omaha' }))
    expect(send).toHaveBeenCalledWith({ type: 'chooseVariant', variant: 'omaha' })
  })
})

describe('Show 2 Play 2', () => {
  it('picks exactly two cards to show', async () => {
    const { ShowPicker } = await import('./ShowPicker.tsx')
    const onAct = vi.fn()
    render(<ShowPicker hole={['As', 'Kd', '7c', '2h']} count={2} onAct={onAct} />)
    const confirm = screen.getByRole('button', { name: 'Pick 2 more' })
    expect(confirm).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: 'Ace of spades' }))
    fireEvent.click(screen.getByRole('button', { name: 'Seven of clubs' }))
    fireEvent.click(screen.getByRole('button', { name: 'Two of hearts' })) // a third is ignored
    expect(screen.getByRole('button', { name: 'Two of hearts' })).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(screen.getByRole('button', { name: 'Seven of clubs' })) // unpick
    fireEvent.click(screen.getByRole('button', { name: 'King of diamonds' }))
    fireEvent.click(screen.getByRole('button', { name: 'Show these 2' }))
    expect(onAct).toHaveBeenCalledWith({ type: 'show', cards: ['As', 'Kd'] })
  })

  it("shows other players' revealed cards face up next to their hidden ones", () => {
    const s = state({
      phase: 'playing',
      stage: 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 4, hole: ['As', 'Kd', '7c', '2h'], shown: ['As', 'Kd'] }),
        player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 4, shown: ['Qh', 'Qd'] }),
        player({ id: 'p3', name: 'Cy', inHand: true, holeCount: 4 }),
      ],
      hand: {
        number: 1,
        variant: 'show2',
        street: 'Pre-flop',
        showing: true,
        reveal: 2,
        boards: [[]],
        pot: 30,
        toAct: 'p3',
        currentBet: 20,
        done: false,
      },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    const bob = screen.getByLabelText('Seat Bob')
    expect(within(bob).getByLabelText('Queen of hearts')).toBeInTheDocument()
    expect(within(bob).getAllByLabelText('Hidden card')).toHaveLength(2)
    expect(within(screen.getByLabelText('Seat Cy')).getAllByLabelText('Hidden card')).toHaveLength(4)
    expect(screen.getByRole('status')).toHaveTextContent('Cy is picking cards to show')
  })
})

describe('Dacoit', () => {
  it('chooses 4 or 6 cards, with 6 disabled when not allowed', async () => {
    const { ChoosePicker } = await import('./ChoosePicker.tsx')
    const onAct = vi.fn()
    const { rerender } = render(<ChoosePicker max={6} onAct={onAct} />)
    fireEvent.click(screen.getByRole('button', { name: /6 cards/ }))
    fireEvent.click(screen.getByRole('button', { name: /4 cards/ }))
    expect(onAct.mock.calls).toEqual([[{ type: 'choose', count: 6 }], [{ type: 'choose', count: 4 }]])

    rerender(<ChoosePicker max={4} onAct={onAct} />)
    expect(screen.getByRole('button', { name: /6 cards/ })).toBeDisabled()
    expect(screen.getByText(/Not enough cards left/)).toBeInTheDocument()
  })

  it('marks 6-card players and shows who is choosing', () => {
    const s = state({
      phase: 'playing',
      stage: 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, dealer: true }),
        player({ id: 'p2', name: 'Bob', inHand: true, picked: 6, mult: 2 }),
        player({ id: 'p3', name: 'Cy', inHand: true }),
      ],
      hand: { number: 1, variant: 'dacoit', street: 'Pre-flop', picking: true, boards: [[]], pot: 0, toAct: 'p3', currentBet: 0, done: false },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(within(screen.getByLabelText('Seat Bob')).getByText('6 cards · 2×')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Cy is choosing 4 or 6 cards')
    // Nobody has cards before the deal, so there's no dock for Ann yet.
    expect(screen.queryByLabelText('Your cards')).toBeNull()
  })

  it('reminds a 6-card player that bets cost double, in chips', () => {
    const s = state({
      phase: 'playing',
      stage: 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, picked: 6, mult: 2, holeCount: 6, hole: ['As', 'Ad', 'Kc', 'Kd', '7h', '2s'], bet: 10 }),
        player({ id: 'p2', name: 'Bob', inHand: true, picked: 4, holeCount: 4, bet: 20 }),
      ],
      hand: {
        number: 1, variant: 'dacoit', street: 'Pre-flop', boards: [[]], pot: 40, toAct: 'p1', currentBet: 20, done: false,
        legal: { fold: true, check: false, call: true, callUnits: 10, bet: false, raise: true, minTo: 40, maxTo: 990 },
      },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getByText(/every bet costs you double/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Call 20' })).toBeInTheDocument() // 10 units x 2
    expect(within(screen.getByLabelText('Seat Ann')).getByText('Bet 20')).toBeInTheDocument()
  })
})

describe('Double/Triple Texas', () => {
  const boards = [
    ['9c', 'Tc', '2d', '3h', 'Ks'],
    ['Ad', '4s', '4h', 'Jc', 'Qd'],
  ]

  it('places two cards per board, lets you take them back, then locks in', async () => {
    const { SplitPicker } = await import('./SplitPicker.tsx')
    const onAct = vi.fn()
    render(<SplitPicker hole={['As', 'Ah', '7c', '8c']} boards={boards} onAct={onAct} />)
    const lock = () => screen.getByRole('button', { name: /Lock in|Place/ })
    expect(lock()).toBeDisabled()

    const hand = () => within(screen.getByLabelText('Cards to place'))
    fireEvent.click(hand().getByRole('button', { name: 'Seven of clubs' })) // board 1
    fireEvent.click(hand().getByRole('button', { name: 'Eight of clubs' })) // board 1
    fireEvent.click(hand().getByRole('button', { name: 'Ace of spades' })) // board 2
    // Changed my mind: put the clubs back, so the aces go to board 1.
    const board1 = within(screen.getByLabelText('Your cards for board 1'))
    fireEvent.click(board1.getByRole('button', { name: 'Seven of clubs' }))
    fireEvent.click(board1.getByRole('button', { name: 'Eight of clubs' }))
    expect(lock()).toHaveTextContent('Place 3 more')

    // Taking a card back aims at that board; move the ace of spades over too.
    const board2 = within(screen.getByLabelText('Your cards for board 2'))
    fireEvent.click(board2.getByRole('button', { name: 'Ace of spades' }))
    fireEvent.click(screen.getByLabelText('Board 1'))
    fireEvent.click(hand().getByRole('button', { name: 'Ace of spades' })) // board 1
    fireEvent.click(hand().getByRole('button', { name: 'Ace of hearts' })) // board 1, now full
    fireEvent.click(hand().getByRole('button', { name: 'Seven of clubs' })) // board 2
    fireEvent.click(hand().getByRole('button', { name: 'Eight of clubs' })) // board 2
    expect(lock()).toBeEnabled()
    fireEvent.click(lock())
    expect(onAct).toHaveBeenCalledWith({ type: 'allocate', cards: ['As', 'Ah', '7c', '8c'] })
  })

  it('shows who has locked in and who we are waiting for', () => {
    const s = state({
      phase: 'playing',
      stage: 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 4, hole: ['As', 'Ah', '7c', '8c'], allocated: true, split: [['As', 'Ah'], ['7c', '8c']] }),
        player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 4, allocated: true }),
        player({ id: 'p3', name: 'Cy', inHand: true, holeCount: 4 }),
      ],
      hand: { number: 1, variant: 'double', street: 'Showdown', allocating: true, boards, pot: 60, currentBet: 0, done: false },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getByRole('status')).toHaveTextContent('Waiting for Cy to split')
    expect(within(screen.getByLabelText('Seat Bob')).getByText('locked in')).toBeInTheDocument()
    expect(screen.getByText('Locked in, waiting for the others')).toBeInTheDocument()
  })

  it('labels each board with its winner after the showdown', () => {
    const s = state({
      phase: 'playing',
      stage: 'result',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 4, handDesc: 'Pair of Aces | Pair of Fours' }),
        player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 4, handDesc: 'King high | Two pair, Kings and Fours' }),
      ],
      hand: { number: 1, variant: 'double', street: 'Showdown', boards, pot: 40, currentBet: 0, done: true, boardWinners: [['p1'], ['p2']] },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    const board = within(screen.getByLabelText('Board'))
    expect(board.getByText('Ann')).toBeInTheDocument()
    expect(board.getByText('Bob')).toBeInTheDocument()
    expect(within(screen.getByLabelText('Seat Bob')).getByText('Two pair, Kings and Fours')).toBeInTheDocument()
  })
})

describe('ResultBanner', () => {
  it('credits each multi-board winner only with the boards they won', async () => {
    const { ResultBanner } = await import('./ResultBanner.tsx')
    const s = state({
      phase: 'playing',
      stage: 'result',
      players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' })],
    })
    const hand = {
      number: 1, variant: 'double', street: 'Showdown', pot: 40, currentBet: 0, done: true,
      boards: [['9s', '5s', '2h', 'Ac', 'Qh'], ['Kc', '7c', 'Tc', '7s', '2s']],
      boardWinners: [['p2'], ['p1']],
      winners: [
        { playerId: 'p1', chips: 20, desc: 'Pair of Fives | Two pair, Kings and Sevens', cards: [['5h', '9s', '5s', 'Ac', 'Qh'], ['Kd', 'Kc', '7c', 'Tc', '7s']] },
        { playerId: 'p2', chips: 20, desc: 'Pair of Queens | Pair of Sevens', cards: [['Qc', '9s', '5s', 'Ac', 'Qh'], ['Js', 'Kc', '7c', 'Tc', '7s']] },
      ],
    }
    render(<ResultBanner state={s} hand={hand} onNext={vi.fn()} />)
    const banner = screen.getByLabelText('Hand result')
    expect(banner).toHaveTextContent('You win 20 (split pot)Board 2: Two pair, Kings and Sevens')
    expect(banner).toHaveTextContent('Bob wins 20 (split pot)Board 1: Pair of Queens')
    expect(banner).not.toHaveTextContent('Pair of Fives')
    expect(within(banner).getAllByLabelText('Winning hand')).toHaveLength(2)
  })
})

describe('Drawer', () => {
  const base = {
    onTab: vi.fn(),
    onClose: vi.fn(),
    history: [] as never[],
    loadHistory: vi.fn(),
    chat: vi.fn(),
    react: vi.fn(),
    prefs: { sound: true, vibrate: true, fourColor: false },
    setPrefs: vi.fn(),
  }

  it('sends chat messages and reactions', async () => {
    const { Drawer } = await import('./Drawer.tsx')
    const s = state({
      chat: [
        { id: 1, playerId: 'p2', name: 'Bob', text: 'nice hand', at: 0 },
        { id: 2, playerId: 'p1', name: 'Ann', text: 'thanks', at: 0 },
      ],
    })
    const chat = vi.fn()
    const react = vi.fn()
    render(<Drawer {...base} tab="chat" state={s} chat={chat} react={react} />)
    const lines = screen.getByLabelText('Messages')
    expect(lines).toHaveTextContent('Bob nice hand')
    expect(lines).toHaveTextContent('You thanks')

    fireEvent.change(screen.getByLabelText('Message'), { target: { value: '  gg  ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    expect(chat).toHaveBeenCalledWith('gg')
    expect(screen.getByLabelText('Message')).toHaveValue('')

    fireEvent.click(screen.getByRole('button', { name: 'React 🔥' }))
    expect(react).toHaveBeenCalledWith('🔥')
  })

  it('loads and shows the hand history', async () => {
    const { Drawer } = await import('./Drawer.tsx')
    const loadHistory = vi.fn()
    const history = [
      { number: 7, variant: 'omaha', boards: [['As', 'Kd', '7c', '2h', '9s']], winners: [{ playerId: 'p2', chips: 120 }], log: ['Bob wins 120'] },
    ]
    render(<Drawer {...base} tab="history" state={state()} history={history} loadHistory={loadHistory} />)
    expect(loadHistory).toHaveBeenCalled()
    expect(screen.getByText(/#7/).closest('summary')).toHaveTextContent('#7 Omaha · Bob +120')
  })

  it('toggles options', async () => {
    const { Drawer } = await import('./Drawer.tsx')
    const setPrefs = vi.fn()
    render(<Drawer {...base} tab="options" state={state()} setPrefs={setPrefs} />)
    fireEvent.click(screen.getByRole('checkbox', { name: /Four-colour deck/ }))
    expect(setPrefs).toHaveBeenCalledWith({ fourColor: true })
  })
})

describe('Table polish', () => {
  it('shows reactions over seats, the hand hint, and who the table is waiting on', () => {
    const now = Date.now()
    const s = state({
      phase: 'playing',
      stage: 'hand',
      now,
      reactions: [
        { id: 1, playerId: 'p2', emoji: '😂', at: now - 500 },
        { id: 2, playerId: 'p1', emoji: '🔥', at: now - 10_000 }, // expired
      ],
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 2, hole: ['As', 'Ad'] }),
        player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 2 }),
      ],
      hand: {
        number: 1, variant: 'holdem', street: 'Flop', boards: [['Ac', '7d', '2s']], pot: 40, toAct: 'p2', currentBet: 0,
        done: false, hint: 'Three of a kind, Aces',
      },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.getByLabelText('Bob reacts 😂')).toBeInTheDocument()
    expect(screen.queryByLabelText('Ann reacts 🔥')).toBeNull()
    expect(screen.getByText('Three of a kind, Aces')).toBeInTheDocument()
    expect(within(screen.getByLabelText('Seat Bob')).getByLabelText('Waiting for Bob')).toBeInTheDocument()
    expect(screen.queryByText(/\d+s$/)).toBeNull() // no countdowns anywhere
  })
})

describe('ActionBar shortcuts', () => {
  it('F folds, C calls, R focuses the raise box, but not while typing', () => {
    const onAct = vi.fn()
    const legal: Legal = { fold: true, check: false, call: true, callUnits: 20, bet: false, raise: true, minTo: 40, maxTo: 1000 }
    render(<ActionBar legal={legal} bet={0} mult={1} pot={30} onAct={onAct} />)
    fireEvent.keyDown(window, { key: 'c' })
    fireEvent.keyDown(window, { key: 'F' })
    expect(onAct.mock.calls).toEqual([[{ type: 'call' }], [{ type: 'fold' }]])

    fireEvent.keyDown(window, { key: 'r' })
    const box = screen.getByLabelText('raise to')
    expect(box).toHaveFocus()
    fireEvent.keyDown(box, { key: 'c' }) // typing in the box is not a shortcut
    expect(onAct).toHaveBeenCalledTimes(2)
  })
})

describe('No timers: Next hand and Skip', () => {
  const waitingOnBob = (you: string) =>
    state({
      phase: 'playing',
      stage: 'hand',
      you,
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 2, hole: you === 'p1' ? ['As', 'Ad'] : undefined }),
        player({ id: 'p2', name: 'Bob', inHand: true, holeCount: 2, hole: you === 'p2' ? ['Kc', 'Kd'] : undefined }),
      ],
      hand: { number: 1, variant: 'holdem', street: 'Flop', boards: [['2c', '7d', '9h']], pot: 40, toAct: 'p2', currentBet: 0, done: false },
    })

  it('lets the host skip the player the table is waiting on, after confirming', () => {
    const send = vi.fn()
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<Table state={waitingOnBob('p1')} clockOffset={0} onAct={vi.fn()} send={send} />)
    expect(within(screen.getByLabelText('Seat Ann')).queryByRole('button', { name: 'Skip' })).toBeNull()
    fireEvent.click(within(screen.getByLabelText('Seat Bob')).getByRole('button', { name: 'Skip' }))
    expect(confirm).toHaveBeenCalled()
    expect(send).toHaveBeenCalledWith({ type: 'skip', target: 'p2' })
    confirm.mockRestore()
  })

  it('does not offer Skip to other players', () => {
    render(<Table state={{ ...waitingOnBob('p2'), hostId: 'p1' }} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.queryByRole('button', { name: 'Skip' })).toBeNull()
  })

  it('anyone can start the next hand from the result', () => {
    const send = vi.fn()
    const s = state({
      phase: 'playing',
      stage: 'result',
      you: 'p2',
      players: [player({ id: 'p1', name: 'Ann', inHand: true, won: 40 }), player({ id: 'p2', name: 'Bob', inHand: true })],
      hand: {
        number: 1, variant: 'holdem', street: 'Showdown', boards: [['2c', '7d', '9h', 'Js', 'Qc']], pot: 40, currentBet: 0, done: true,
        winners: [{ playerId: 'p1', chips: 40 }],
      },
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={send} />)
    fireEvent.click(screen.getByRole('button', { name: 'Next hand' }))
    expect(send).toHaveBeenCalledWith({ type: 'nextHand' })
  })
})

describe('Game over', () => {
  const ended = (you: string) =>
    state({
      phase: 'ended',
      you,
      players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' })],
      leaderboard: [
        { id: 'p1', name: 'Ann', profit: 300 },
        { id: 'p2', name: 'Bob', profit: -300 },
      ],
    })

  it('lets the host start a new game with the same players', () => {
    const send = vi.fn()
    render(<Table state={ended('p1')} clockOffset={0} onAct={vi.fn()} send={send} />)
    expect(screen.getByText('Game over')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'New game with same players' }))
    expect(send).toHaveBeenCalledWith({ type: 'rematch' })
  })

  it('tells everyone else the host can start one', () => {
    render(<Table state={ended('p2')} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.queryByRole('button', { name: 'New game with same players' })).toBeNull()
    expect(screen.getByText(/Ann \(the host\) can start a new game/)).toBeInTheDocument()
  })
})

describe('Vote to kick', () => {
  const three = (over: Partial<ReturnType<typeof state>> = {}) =>
    state({
      phase: 'playing',
      stage: 'result',
      players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' }), player({ id: 'p3', name: 'Cy' })],
      ...over,
    })

  it('starts a vote from a seat after confirming, but not against yourself', () => {
    const send = vi.fn()
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<Table state={three()} clockOffset={0} onAct={vi.fn()} send={send} />)
    expect(within(screen.getByLabelText('Seat Ann')).queryByRole('button', { name: /remove/ })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Vote to remove Cy' }))
    expect(confirm).toHaveBeenCalled()
    expect(send).toHaveBeenCalledWith({ type: 'voteKick', target: 'p3', yes: true })
    confirm.mockRestore()
  })

  it('lets everyone but the target answer, and the starter cancel', () => {
    const send = vi.fn()
    const vote = { target: 'p3', startedBy: 'p1', yes: ['p1'], no: [], needed: 2 }
    const { rerender } = render(<KickVotes state={three({ you: 'p2', kickVotes: [vote] })} send={send} />)
    const group = screen.getByRole('group', { name: 'Vote to remove Cy' })
    expect(group).toHaveTextContent('1 of 2 yes')
    expect(within(group).queryByRole('button', { name: 'Cancel' })).toBeNull()
    fireEvent.click(within(group).getByRole('button', { name: 'No' }))
    expect(send).toHaveBeenCalledWith({ type: 'voteKick', target: 'p3', yes: false })

    rerender(<KickVotes state={three({ you: 'p1', kickVotes: [vote] })} send={send} />)
    expect(screen.getByRole('button', { name: 'Yes' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(send).toHaveBeenCalledWith({ type: 'cancelKick', target: 'p3' })

    rerender(<KickVotes state={three({ you: 'p3', kickVotes: [vote] })} send={send} />)
    expect(screen.getByRole('status')).toHaveTextContent('voting on removing you')
    expect(screen.queryByRole('button', { name: 'Yes' })).toBeNull()
  })

  it('hides removed players from the table but keeps them on the leaderboard', () => {
    const s = three({
      players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' }), player({ id: 'p3', name: 'Cy', kicked: true })],
      leaderboard: [
        { id: 'p3', name: 'Cy', profit: 500, kicked: true },
        { id: 'p1', name: 'Ann', profit: 0 },
        { id: 'p2', name: 'Bob', profit: -500 },
      ],
    })
    render(<Table state={s} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    expect(screen.queryByLabelText('Seat Cy')).toBeNull()
    const lb = screen.getByRole('heading', { name: 'Leaderboard' }).parentElement!
    expect(lb).toHaveTextContent('Cyremoved')
  })
})

describe('Rearranging your cards', () => {
  const setup = () => {
    const onMove = vi.fn()
    const onSwap = vi.fn()
    const onSort = vi.fn()
    render(<HandCards cards={['7h', 'As', '7c', 'Kh']} nextSort="rank" onMove={onMove} onSwap={onSwap} onSort={onSort} />)
    const card = (name: string) => screen.getByRole('button', { name })
    return { onMove, onSwap, onSort, card }
  }
  const tap = (el: HTMLElement) => {
    fireEvent.pointerDown(el, { button: 0, clientX: 10, clientY: 10 })
    fireEvent.pointerUp(el, { button: 0, clientX: 10, clientY: 10 })
  }

  it('tap one card then another to swap them', () => {
    const { onSwap, card } = setup()
    tap(card('Seven of hearts'))
    expect(card('Seven of hearts')).toHaveAttribute('aria-pressed', 'true')
    tap(card('King of hearts'))
    expect(onSwap).toHaveBeenCalledWith(0, 3)
    expect(card('Seven of hearts')).toHaveAttribute('aria-pressed', 'false')
  })

  it('tapping the picked card again puts it back', () => {
    const { onSwap, card } = setup()
    tap(card('Ace of spades'))
    tap(card('Ace of spades'))
    expect(onSwap).not.toHaveBeenCalled()
    expect(card('Ace of spades')).toHaveAttribute('aria-pressed', 'false')
  })

  it('drag a card onto another spot to move it there', () => {
    const { onMove, card } = setup()
    const target = card('Seven of clubs')
    const real = document.elementsFromPoint
    document.elementsFromPoint = () => [target]
    try {
      const from = card('Seven of hearts')
      fireEvent.pointerDown(from, { button: 0, clientX: 0, clientY: 0 })
      fireEvent.pointerMove(from, { clientX: 60, clientY: 0 })
      fireEvent.pointerUp(from, { clientX: 60, clientY: 0 })
    } finally {
      document.elementsFromPoint = real
    }
    expect(onMove).toHaveBeenCalledWith(0, 2)
  })

  it('arrow keys move the picked card', () => {
    const { onMove, card } = setup()
    tap(card('Ace of spades'))
    fireEvent.keyDown(card('Ace of spades'), { key: 'ArrowRight' })
    expect(onMove).toHaveBeenCalledWith(1, 2)
  })

  it('offers the next sort', () => {
    const { onSort } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Sort by rank' }))
    expect(onSort).toHaveBeenCalled()
  })
})

describe('Show 2 Play 2: shown and hidden cards stay apart', () => {
  const s2p2 = (over: Partial<ReturnType<typeof state>> = {}, done = false) =>
    state({
      phase: 'playing',
      stage: done ? 'result' : 'hand',
      players: [
        player({ id: 'p1', name: 'Ann', inHand: true, holeCount: 4, hole: ['7c', 'As', '2h', 'Kd'], shown: ['As', 'Kd'] }),
        player({
          id: 'p2',
          name: 'Bob',
          inHand: true,
          holeCount: 4,
          shown: ['Qh', 'Qd'],
          ...(done ? { hole: ['9c', 'Qh', '3s', 'Qd'] } : {}),
        }),
      ],
      hand: { number: 1, variant: 'show2', street: done ? 'Showdown' : 'Flop', boards: [['2c', '7d', '9h']], pot: 40, toAct: 'p2', currentBet: 0, done },
      ...over,
    })
  const names = (el: HTMLElement) => within(el).queryAllByLabelText(/ of |Hidden card/).map((c) => c.getAttribute('aria-label'))

  it('groups your own cards as Shown, then Hidden', () => {
    render(<Table state={s2p2()} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    const dock = screen.getByRole('group', { name: 'Your cards' })
    expect(names(within(dock).getByRole('group', { name: 'Shown' }))).toEqual(['Ace of spades', 'King of diamonds'])
    expect(names(within(dock).getByRole('group', { name: 'Hidden' }))).toEqual(['Seven of clubs', 'Two of hearts'])
  })

  it("groups others' shown cards apart from their face-down ones", () => {
    render(<Table state={s2p2()} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    const bob = screen.getByLabelText('Seat Bob')
    expect(names(within(bob).getByLabelText('Shown'))).toEqual(['Queen of hearts', 'Queen of diamonds'])
    expect(names(within(bob).getByLabelText('Hidden'))).toEqual(['Hidden card', 'Hidden card'])
  })

  it('keeps the groups at the showdown, when the hidden cards are revealed', () => {
    render(<Table state={s2p2({}, true)} clockOffset={0} onAct={vi.fn()} send={vi.fn()} />)
    const bob = screen.getByLabelText('Seat Bob')
    expect(names(within(bob).getByLabelText('Shown'))).toEqual(['Queen of hearts', 'Queen of diamonds'])
    expect(names(within(bob).getByLabelText('Hidden'))).toEqual(['Nine of clubs', 'Three of spades'])
  })

  it('moves cards only within their group', () => {
    const onSwap = vi.fn()
    const onMove = vi.fn()
    render(<HandCards cards={['7c', 'As', '2h', 'Kd']} shown={['As', 'Kd']} nextSort="rank" onMove={onMove} onSwap={onSwap} onSort={vi.fn()} />)
    const card = (name: string) => screen.getByRole('button', { name })
    const tap = (el: HTMLElement) => {
      fireEvent.pointerDown(el, { button: 0 })
      fireEvent.pointerUp(el, { button: 0 })
    }
    // Shown → hidden: the pick just moves to the hidden card.
    tap(card('Ace of spades'))
    tap(card('Seven of clubs'))
    expect(onSwap).not.toHaveBeenCalled()
    expect(card('Seven of clubs')).toHaveAttribute('aria-pressed', 'true')
    // Hidden → hidden swaps (indexes into the hand: 7c is 0, 2h is 2).
    tap(card('Two of hearts'))
    expect(onSwap).toHaveBeenCalledWith(0, 2)
    // Arrow keys stay in the group: Ace's right-hand neighbour is the King.
    tap(card('Ace of spades'))
    fireEvent.keyDown(card('Ace of spades'), { key: 'ArrowRight' })
    expect(onMove).toHaveBeenCalledWith(1, 3)
    fireEvent.keyDown(card('Ace of spades'), { key: 'ArrowLeft' })
    expect(onMove).toHaveBeenCalledTimes(1) // already first in its group
  })
})
