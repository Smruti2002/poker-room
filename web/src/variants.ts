export const VARIANT_NAMES: Record<string, string> = {
  holdem: "Texas Hold'em",
  omaha: 'Omaha',
  dacoit: 'Dacoit',
  double: 'Double Texas',
  triple: 'Triple Texas',
  show2: 'Show 2 Play 2',
  bb: 'BB',
}

export function variantName(id: string): string {
  return VARIANT_NAMES[id] ?? id
}
