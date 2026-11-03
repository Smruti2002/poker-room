import { type FormEvent, useEffect, useRef, useState } from 'react'
import type { Prefs } from '../prefs.ts'
import type { HistoryHand, State } from '../protocol/types.ts'
import { EMOJIS } from '../emojis.ts'
import { variantName } from '../variants.ts'
import { Card } from './Card.tsx'

export type DrawerTab = 'chat' | 'history' | 'options'

interface Props {
  tab: DrawerTab
  onTab: (t: DrawerTab) => void
  onClose: () => void
  state: State
  history: HistoryHand[] | null
  loadHistory: () => void
  chat: (text: string) => void
  react: (emoji: string) => void
  prefs: Prefs
  setPrefs: (p: Partial<Prefs>) => void
}

/** Chat, hand history and options: a bottom sheet on phones, a side panel on desktop. */
export function Drawer(props: Props) {
  const { tab, onTab, onClose } = props
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <>
      <div className="drawer-backdrop" onClick={onClose} />
      <aside className="drawer" role="dialog" aria-label="Chat and history">
        <div className="drawer-tabs" role="tablist">
          {(['chat', 'history', 'options'] as const).map((t) => (
            <button
              key={t}
              role="tab"
              aria-selected={tab === t}
              className={`drawer-tab${tab === t ? ' drawer-tab-on' : ''}`}
              onClick={() => onTab(t)}
            >
              {t === 'chat' ? 'Chat' : t === 'history' ? 'History' : 'Options'}
            </button>
          ))}
          <button className="btn btn-link drawer-close" aria-label="Close" onClick={onClose}>
            ✕
          </button>
        </div>
        {tab === 'chat' && <ChatPanel {...props} />}
        {tab === 'history' && <HistoryPanel {...props} />}
        {tab === 'options' && <OptionsPanel {...props} />}
      </aside>
    </>
  )
}

function ChatPanel({ state, chat, react }: Props) {
  const [text, setText] = useState('')
  const list = useRef<HTMLOListElement>(null)
  // Keep the newest message in view. (Not scrollIntoView: newer browsers
  // return a Promise from it, and an effect must not return one.)
  useEffect(() => {
    if (list.current) list.current.scrollTop = list.current.scrollHeight
  }, [state.chat.length])

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!text.trim()) return
    chat(text.trim())
    setText('')
  }

  return (
    <div className="chat">
      <div className="emoji-bar" aria-label="Reactions">
        {EMOJIS.map((e) => (
          <button key={e} type="button" className="emoji-btn" onClick={() => react(e)} aria-label={`React ${e}`}>
            {e}
          </button>
        ))}
      </div>
      <ol className="chat-lines" aria-label="Messages" ref={list}>
        {state.chat.length === 0 && <li className="muted">No messages yet. Say hi!</li>}
        {state.chat.map((line) => (
          <li key={line.id} className={line.playerId === state.you ? 'chat-mine' : undefined}>
            <span className="chat-name">{line.playerId === state.you ? 'You' : line.name}</span> {line.text}
          </li>
        ))}
      </ol>
      <form className="chat-form" onSubmit={submit}>
        <input
          value={text}
          onChange={(e) => setText(e.target.value)}
          maxLength={200}
          placeholder="Message the table"
          aria-label="Message"
        />
        <button className="btn btn-primary" disabled={!text.trim()}>
          Send
        </button>
      </form>
    </div>
  )
}

function HistoryPanel({ state, history, loadHistory }: Props) {
  // Fetch fresh each time the tab opens or a hand finishes.
  const handNo = state.hand?.done ? state.hand.number : (state.hand?.number ?? 0) - 1
  useEffect(() => {
    loadHistory()
  }, [handNo]) // eslint-disable-line react-hooks/exhaustive-deps
  const name = (id: string) => (id === state.you ? 'You' : (state.players.find((p) => p.id === id)?.name ?? '?'))

  if (history === null) return <p className="muted">Loading…</p>
  if (history.length === 0) return <p className="muted">No finished hands yet.</p>
  return (
    <ol className="history">
      {history.map((h) => (
        <li key={h.number}>
          <details>
            <summary>
              <span className="history-num">#{h.number}</span> {variantName(h.variant)} ·{' '}
              {h.winners.map((w) => `${name(w.playerId)} +${w.chips}`).join(', ')}
            </summary>
            <div className="history-boards">
              {h.boards.map((b, i) => (
                <div className="history-board" key={i}>
                  {b.map((c) => (
                    <Card key={c} code={c} small />
                  ))}
                </div>
              ))}
            </div>
            <ol className="log">
              {h.log.map((line, i) => (
                <li key={i}>{line}</li>
              ))}
            </ol>
          </details>
        </li>
      ))}
    </ol>
  )
}

function OptionsPanel({ prefs, setPrefs }: Props) {
  const toggle = (key: keyof Prefs, label: string, hint: string) => (
    <label className="option">
      <input type="checkbox" checked={prefs[key]} onChange={(e) => setPrefs({ [key]: e.target.checked })} />
      <span>
        {label}
        <span className="muted small"> {hint}</span>
      </span>
    </label>
  )
  return (
    <div className="options">
      {toggle('sound', 'Sound', 'Chips, your turn, and wins')}
      {toggle('vibrate', 'Vibrate on your turn', 'On phones that support it')}
      {toggle('fourColor', 'Four-colour deck', 'Clubs green, diamonds blue')}
      <p className="muted small">These settings are saved on this device only.</p>
    </div>
  )
}
