export type Color = 'white' | 'black'

export interface Database {
  games: number
  skipped: { number: number; reason: string }[]
  /** Whether engine evaluations and the mistake scan are available. */
  engine: boolean
}

export interface Player {
  name: string
  games: number
}

/** One move played from a position. Wins, draws, losses and score are the opponent's. */
export interface Move {
  san: string
  uci: string
  /** Key of the position after the move. */
  position: string
  /** Raw number of games. */
  games: number
  /** Share of this move among all moves from the position, with older games discounted. */
  share: number
  weight: number
  wins: number
  draws: number
  losses: number
  /** Opponent's points per finished game, 0 to 1. Missing when no game has a result. */
  score?: number
  /** ISO date of the most recent dated game. */
  lastPlayed?: string
}

export interface TreeNode {
  position: string
  sideToMove: Color
  /** Whose choice this position is. */
  mover: 'you' | 'opponent'
  games: number
  moves: Move[]
}

/** An engine evaluation from your point of view. */
export interface Eval {
  /** "+0.31" in pawns, "#3" when you mate in three, "-#3" when you get mated. */
  text: string
  /** Your advantage in centipawns, capped at 1000 either way. */
  centipawns: number
  depth: number
}

export interface Analysis {
  position: string
  eval: Eval
  /** The engine's preferred move in this position. */
  best?: string
  moves: {
    uci: string
    eval: Eval
    /** Centipawns the move gives away for whoever plays it. */
    loss: number
  }[]
}

export interface Step {
  san: string
  uci: string
  position: string
}

/** A habitual move of the opponent that the engine dislikes. */
export interface Mistake {
  /** Moves from the start to the position where the opponent errs. */
  line: Step[]
  position: string
  nodeGames: number
  move: Move
  /** Centipawns the move gives away. */
  loss: number
  /** What the engine would have played instead. */
  best?: string
  /** The position after their move, from your point of view. */
  eval: Eval
  /** How the engine suggests you continue. */
  refutation: string[]
}

export interface Scan {
  status: 'running' | 'done' | 'failed'
  done: number
  total: number
  error?: string
  items: Mistake[]
}

async function get<T>(path: string, params: Record<string, string> = {}): Promise<T> {
  const query = new URLSearchParams(params).toString()
  const response = await fetch(query ? `${path}?${query}` : path)
  if (!response.ok) {
    const problem = await response.json().catch(() => null)
    throw new Error(problem?.detail ?? `${path} failed with status ${response.status}`)
  }
  return response.json()
}

export function fetchDatabase(): Promise<Database> {
  return get('/api/v1/database')
}

export async function searchPlayers(query: string): Promise<Player[]> {
  const body = await get<{ items: Player[] }>('/api/v1/players', { query, limit: '8' })
  return body.items
}

export function fetchNode(
  opponent: string,
  myColor: Color,
  halfLifeYears: number,
  position?: string,
): Promise<TreeNode> {
  const params: Record<string, string> = {
    opponent,
    myColor,
    halfLifeYears: String(halfLifeYears),
  }
  if (position) params.position = position
  return get('/api/v1/tree', params)
}

export function fetchAnalysis(
  opponent: string,
  myColor: Color,
  halfLifeYears: number,
  position?: string,
): Promise<Analysis> {
  const params: Record<string, string> = {
    opponent,
    myColor,
    halfLifeYears: String(halfLifeYears),
  }
  if (position) params.position = position
  return get('/api/v1/analysis', params)
}

/** Starts the scan on the first call and reports its progress on later ones. */
export function fetchMistakes(opponent: string, myColor: Color, halfLifeYears: number): Promise<Scan> {
  return get('/api/v1/mistakes', { opponent, myColor, halfLifeYears: String(halfLifeYears) })
}
