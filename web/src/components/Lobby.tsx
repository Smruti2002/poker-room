import { useState } from 'react'
import type { Settings, State } from '../protocol/types.ts'
import { variantName } from '../variants.ts'

interface Props {
  state: State
  onSave: (s: Settings) => void
  onStart: () => void
}

type NumberField = Exclude<keyof Settings, 'variants' | 'potLimit' | 'hints'>

const FIELDS: { key: NumberField; label: string; min: number; max: number; hint?: string }[] = [
  { key: 'startingStack', label: 'Starting stack', min: 100, max: 1_000_000 },
  { key: 'loanAmount', label: 'Loan amount', min: 100, max: 1_000_000 },
  { key: 'loanPenalty', label: 'Loan penalty', min: 0, max: 1_000_000 },
  { key: 'penalizedLoans', label: 'Penalized loans', min: 0, max: 100, hint: 'Loans after this many carry no penalty' },
  { key: 'smallBlind', label: 'Small blind', min: 1, max: 1_000_000 },
  { key: 'bigBlind', label: 'Big blind', min: 1, max: 1_000_000 },
  { key: 'turnSeconds', label: 'Turn timer (s)', min: 10, max: 120 },
  { key: 'maxPlayers', label: 'Max players', min: 2, max: 10 },
]

export function Lobby({ state, onSave, onStart }: Props) {
  const isHost = state.you === state.hostId
  const [draft, setDraft] = useState<Settings>(state.settings)
  const dirty = isHost && JSON.stringify(draft) !== JSON.stringify(state.settings)
  // Only the host edits; everyone else sees the live settings.
  const shown = isHost ? draft : state.settings
  const link = `${window.location.origin}/r/${state.code}`
  const [copied, setCopied] = useState(false)

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(link)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // Clipboard may be blocked; the link is visible to copy by hand.
    }
  }

  const toggleVariant = (v: string) =>
    setDraft((d) => ({
      ...d,
      variants: d.variants.includes(v) ? d.variants.filter((x) => x !== v) : [...d.variants, v],
    }))

  return (
    <div className="lobby">
      <section className="panel">
        <h2>Invite friends</h2>
        <div className="share">
          <code className="share-link">{link}</code>
          <button className="btn btn-small" onClick={copy}>
            {copied ? 'Copied' : 'Copy link'}
          </button>
        </div>
        <h3>
          Players ({state.players.length}/{state.settings.maxPlayers})
        </h3>
        <ul className="lobby-players">
          {state.players.map((p) => (
            <li key={p.id}>
              {p.name}
              {p.id === state.hostId && <span className="tag">host</span>}
              {p.id === state.you && <span className="tag">you</span>}
              {!p.connected && <span className="tag tag-muted">offline</span>}
            </li>
          ))}
        </ul>
        {isHost ? (
          <button
            className="btn btn-primary"
            disabled={state.players.length < 2 || dirty}
            onClick={onStart}
            title={dirty ? 'Save settings first' : undefined}
          >
            Start game
          </button>
        ) : (
          <p className="muted">Waiting for the host to start…</p>
        )}
      </section>

      <section className="panel">
        <h2>Settings</h2>
        <form
          className="settings"
          onSubmit={(e) => {
            e.preventDefault()
            onSave(draft)
          }}
        >
          {FIELDS.map((f) => (
            <label key={f.key} title={f.hint}>
              <span>{f.label}</span>
              <input
                type="number"
                min={f.min}
                max={f.max}
                value={shown[f.key]}
                disabled={!isHost}
                onChange={(e) => setDraft({ ...draft, [f.key]: Number(e.target.value) })}
              />
            </label>
          ))}
          <fieldset disabled={!isHost}>
            <legend>Variants the dealer can pick</legend>
            {state.variants.map((v) => (
              <label key={v} className="check">
                <input type="checkbox" checked={shown.variants.includes(v)} onChange={() => toggleVariant(v)} />
                {variantName(v)}
              </label>
            ))}
            <label className="check">
              <input
                type="checkbox"
                checked={shown.potLimit}
                onChange={(e) => setDraft({ ...draft, potLimit: e.target.checked })}
              />
              Pot-limit for Omaha, Dacoit and Show 2 Play 2
            </label>
            <label className="check">
              <input
                type="checkbox"
                checked={shown.hints}
                onChange={(e) => setDraft({ ...draft, hints: e.target.checked })}
              />
              Show players the name of their current hand
            </label>
          </fieldset>
          {isHost && (
            <div className="settings-actions">
              <button className="btn" type="submit" disabled={!dirty}>
                Save settings
              </button>
              {dirty && (
                <button className="btn btn-link" type="button" onClick={() => setDraft(state.settings)}>
                  Reset
                </button>
              )}
            </div>
          )}
        </form>
      </section>
    </div>
  )
}
