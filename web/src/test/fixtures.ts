import type { PlayerView, Settings, State } from '../protocol/types.ts'

export const settings: Settings = {
  startingStack: 2000,
  loanAmount: 2000,
  loanPenalty: 1000,
  penalizedLoans: 3,
  smallBlind: 10,
  bigBlind: 20,
  potLimit: false,
  variants: ['holdem'],
  maxPlayers: 8,
  hints: true,
}

export function player(over: Partial<PlayerView> & { id: string; name: string }): PlayerView {
  return {
    stack: 2000,
    debt: 0,
    loans: 0,
    profit: 0,
    connected: true,
    sittingOut: false,
    broke: false,
    inHand: false,
    dealer: false,
    folded: false,
    allIn: false,
    bet: 0,
    mult: 1,
    ...over,
  }
}

export function state(over: Partial<State> = {}): State {
  return {
    code: 'ABCDEF',
    phase: 'lobby',
    you: 'p1',
    hostId: 'p1',
    settings,
    players: [player({ id: 'p1', name: 'Ann' }), player({ id: 'p2', name: 'Bob' })],
    log: [],
    leaderboard: [],
    variants: ['holdem'],
    chat: [],
    reactions: [],
    kickVotes: [],
    now: Date.now(),
    ...over,
  }
}
