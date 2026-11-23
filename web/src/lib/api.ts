export type Color = 'white' | 'black'

export interface Database {
  games: number
  skipped: { number: number; reason: string }[]
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
