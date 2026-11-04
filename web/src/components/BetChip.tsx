/** Chip colours by amount, like a casino's; All-in is gold. */
const COLORS: Record<string, { face: string; text: string }> = {
  '50': { face: '#2f6fd0', text: '#fff' },
  '100': { face: '#262626', text: '#fff' },
  '150': { face: '#2e8b57', text: '#fff' },
  '250': { face: '#c0392b', text: '#fff' },
  '500': { face: '#7b3fb0', text: '#fff' },
  allin: { face: '#e0b23a', text: '#1b1b1b' },
}

/**
 * A poker chip: a coloured disc with the classic stripes round the edge and
 * the amount in the middle. Drawn as SVG, so it's sharp at any size.
 */
export function ChipFace({ label, color }: { label: string; color: string }) {
  const c = COLORS[color] ?? COLORS.allin
  const small = label.length > 3
  return (
    <svg viewBox="0 0 48 48" className="chip-face" aria-hidden="true">
      <circle cx="24" cy="24" r="23" fill={c.face} />
      {/* Edge stripes: a thick dashed ring. */}
      <circle cx="24" cy="24" r="19.5" fill="none" stroke="#fff" strokeOpacity="0.9" strokeWidth="5" strokeDasharray="6.1 6.15" />
      <circle cx="24" cy="24" r="15" fill={c.face} stroke="#fff" strokeOpacity="0.5" strokeWidth="1" strokeDasharray="2 2" />
      <text
        x="24"
        y="24"
        textAnchor="middle"
        dominantBaseline="central"
        fill={c.text}
        fontSize={small ? 8.5 : 12}
        fontWeight="800"
        fontFamily="inherit"
      >
        {label}
      </text>
    </svg>
  )
}
