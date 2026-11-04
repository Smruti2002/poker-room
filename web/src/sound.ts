// Small synthesized sounds, so there are no audio files to download. Browsers
// only allow audio after the user has interacted with the page, so the
// context is created lazily and failures are ignored.

let ctx: AudioContext | null = null

function audio(): AudioContext | null {
  try {
    ctx ??= new AudioContext()
    if (ctx.state === 'suspended') void ctx.resume()
    return ctx
  } catch {
    return null
  }
}

function tone(freq: number, start: number, length: number, volume = 0.15, type: OscillatorType = 'sine') {
  const ac = audio()
  if (!ac) return
  const t = ac.currentTime + start
  const osc = ac.createOscillator()
  const gain = ac.createGain()
  osc.type = type
  osc.frequency.value = freq
  gain.gain.setValueAtTime(volume, t)
  gain.gain.exponentialRampToValueAtTime(0.001, t + length)
  osc.connect(gain).connect(ac.destination)
  osc.start(t)
  osc.stop(t + length)
}

export const sounds = {
  /** Two rising notes: it's your turn. */
  turn() {
    tone(660, 0, 0.15)
    tone(880, 0.12, 0.2)
  },
  /** A short click: chips went in. */
  chip() {
    tone(1800, 0, 0.05, 0.08, 'triangle')
    tone(2400, 0.03, 0.04, 0.05, 'triangle')
  },
  /** A firmer double knock: someone bet or raised. */
  raise() {
    tone(330, 0, 0.09, 0.14, 'square')
    tone(440, 0.1, 0.12, 0.12, 'square')
  },
  /** A little arpeggio: you won. */
  win() {
    ;[523, 659, 784, 1047].forEach((f, i) => tone(f, i * 0.09, 0.25, 0.12))
  },
}
