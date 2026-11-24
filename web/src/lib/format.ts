/** "1. e4" for White's moves; for Black's "c5", or "1... c5" when `always` is set. */
export function numbered(ply: number, san: string, always = false): string {
  const number = Math.floor(ply / 2) + 1
  if (ply % 2 === 0) return `${number}. ${san}`
  return always ? `${number}... ${san}` : san
}

/** A run of moves as text, starting at the given half-move (0 is White's first). */
export function moveText(sans: string[], firstPly: number): string {
  return sans.map((san, index) => numbered(firstPly + index, san, index === 0)).join(' ')
}

export const percent = (fraction: number) => `${Math.round(fraction * 100)}%`

/** "2026-03" from an ISO date. */
export const month = (date?: string) => date?.slice(0, 7) ?? ''

/** Centipawns as pawns with one decimal. */
export const pawns = (centipawns: number) => (centipawns / 100).toFixed(1)
