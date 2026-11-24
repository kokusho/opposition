export type Color = 'white' | 'black'

export interface Database {
  games: number
  skipped: { number: number; reason: string }[]
  /** Whether engine evaluations and the mistake scan are available. */
  engine: boolean
  /** Whether human-move predictions (Maia) are available. */
  predictions: boolean
  /** Whether a language model can write the report up. */
  narrative: boolean
}

export interface Player {
  name: string
  games: number
  /** Rating from the player's most recent game that states one. */
  rating?: number
}

/** A move that can be made on the board, with the position it leads to. */
export interface Playable {
  san: string
  uci: string
  position: string
}

export interface LegalMoves {
  position: string
  moves: Playable[]
}

/** What a model expects a human of a given rating to do in a position. */
export interface Prediction {
  position: string
  /** Rating of the side to move and of the other player. */
  selfElo: number
  oppoElo: number
  /** Predicted result for the side to move; the three add up to 1. */
  win: number
  draw: number
  loss: number
  /** Every legal move, most likely first. */
  moves: (Playable & { probability: number })[]
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

/** A line of moves the opponent's games followed repeatedly, with their results. */
export interface MainLine {
  line: Step[]
  games: number
  wins: number
  draws: number
  losses: number
  score?: number
}

/** A move the opponent faced and scored badly against. */
export interface WeakSpot {
  /** Moves leading to the position where you move. */
  line: Step[]
  position: string
  move: Move
}

/** A predicted answer of the opponent to a trap move. */
export interface TrapReply extends Playable {
  probability: number
  /** Centipawns the reply gives away. */
  loss: number
  /** The position after the reply, from your point of view. */
  eval: Eval
  /** The engine's answer to the reply. */
  punish?: string
}

/** A sound move of yours after which a player of their rating often goes wrong. */
export interface Trap {
  /** Moves leading to the position where you move. */
  line: Step[]
  position: string
  /** How many of their games reached that position. */
  games: number
  move: Step
  /** How often they faced the move; 0 means the engine suggested it. */
  facedGames: number
  ownLoss: number
  /** The position after your move, from your point of view. */
  eval: Eval
  /** Probability that their reply loses a pawn or more. */
  errorChance: number
  expectedLoss: number
  replies: TrapReply[]
}

export interface Report {
  opponent: string
  myColor: Color
  theirElo: number
  myElo: number
  summary: {
    games: number
    wins: number
    draws: number
    losses: number
    score?: number
    first?: string
    last?: string
  }
  mainLines: MainLine[]
  weakSpots: WeakSpot[]
  /** Null without a chess engine. */
  mistakes: Mistake[] | null
  /** Null without both a chess engine and a prediction model. */
  traps: Trap[] | null
}

/** A report as far as it has got; the statistics are there from the start. */
export interface ReportState {
  status: 'running' | 'done' | 'failed'
  /** What the engine is working on. */
  stage?: 'mistakes' | 'traps'
  done: number
  total: number
  error?: string
  report: Report
}

async function get<T>(path: string, params: Record<string, string> = {}, method = 'GET'): Promise<T> {
  const query = new URLSearchParams(params).toString()
  const response = await fetch(query ? `${path}?${query}` : path, { method })
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

/** The legal moves of a position (the start position when omitted). */
export function fetchMoves(position?: string): Promise<LegalMoves> {
  return get('/api/v1/moves', position ? { position } : {})
}

/** selfElo is the rating of the side to move, oppoElo of the other player. */
export function fetchPredictions(position: string, selfElo: number, oppoElo: number): Promise<Prediction> {
  return get('/api/v1/predictions', { position, selfElo: String(selfElo), oppoElo: String(oppoElo) })
}

function reportParams(opponent: string, myColor: Color, halfLifeYears: number, theirElo?: number, myElo?: number) {
  const params: Record<string, string> = { opponent, myColor, halfLifeYears: String(halfLifeYears) }
  if (theirElo) params.theirElo = String(theirElo)
  if (myElo) params.myElo = String(myElo)
  return params
}

/** Starts the report on the first call and returns its progress on later ones. */
export function fetchReport(
  opponent: string,
  myColor: Color,
  halfLifeYears: number,
  theirElo?: number,
  myElo?: number,
): Promise<ReportState> {
  return get('/api/v1/report', reportParams(opponent, myColor, halfLifeYears, theirElo, myElo))
}

/** Has the language model write up a finished report. */
export async function fetchNarrative(
  opponent: string,
  myColor: Color,
  halfLifeYears: number,
  theirElo?: number,
  myElo?: number,
): Promise<string> {
  const params = reportParams(opponent, myColor, halfLifeYears, theirElo, myElo)
  const body = await get<{ text: string }>('/api/v1/report/narrative', params, 'POST')
  return body.text
}
