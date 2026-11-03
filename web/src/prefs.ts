import { useEffect, useState } from 'react'

/** Per-device preferences, kept in localStorage. */
export interface Prefs {
  sound: boolean
  vibrate: boolean
  /** Four-colour deck: clubs green, diamonds blue. */
  fourColor: boolean
}

const KEY = 'poker-room:prefs'
const DEFAULTS: Prefs = { sound: true, vibrate: true, fourColor: false }

function load(): Prefs {
  try {
    return { ...DEFAULTS, ...(JSON.parse(localStorage.getItem(KEY) ?? '{}') as Partial<Prefs>) }
  } catch {
    return DEFAULTS
  }
}

export function usePrefs(): [Prefs, (p: Partial<Prefs>) => void] {
  const [prefs, setPrefs] = useState<Prefs>(load)
  useEffect(() => {
    try {
      localStorage.setItem(KEY, JSON.stringify(prefs))
    } catch {
      // storage may be unavailable; prefs then last for this visit only
    }
    document.body.classList.toggle('four-color', prefs.fourColor)
  }, [prefs])
  return [prefs, (p) => setPrefs((old) => ({ ...old, ...p }))]
}
