<script lang="ts">
  import { Chessground } from '@lichess-org/chessground'
  import type { Api } from '@lichess-org/chessground/api'
  import type { Config } from '@lichess-org/chessground/config'
  import type { Key } from '@lichess-org/chessground/types'
  import '@lichess-org/chessground/assets/chessground.base.css'
  import '@lichess-org/chessground/assets/chessground.brown.css'
  import '@lichess-org/chessground/assets/chessground.cburnett.css'
  import type { Color, Playable } from './api'

  interface Props {
    /** Position to show. Only the piece placement field of the key is used. */
    position: string
    orientation: Color
    sideToMove: Color
    /** The moves that can be played on the board from this position. */
    moves: Playable[]
    /** UCI of the move that led here, for highlighting. */
    lastMove?: string
    onmove: (move: Playable) => void
  }

  let { position, orientation, sideToMove, moves, lastMove, onmove }: Props = $props()

  let element: HTMLDivElement
  let ground: Api | undefined

  const squares = (uci: string): [Key, Key] => [uci.slice(0, 2) as Key, uci.slice(2, 4) as Key]

  function config(): Config {
    const dests = new Map<Key, Key[]>()
    for (const move of moves) {
      const [from, to] = squares(move.uci)
      dests.set(from, [...(dests.get(from) ?? []), to])
    }
    return {
      fen: position,
      orientation,
      turnColor: sideToMove,
      lastMove: lastMove ? squares(lastMove) : undefined,
      movable: { free: false, color: sideToMove, dests, showDests: true },
      premovable: { enabled: false },
      draggable: { showGhost: true },
      events: {
        move: (from, to) => {
          // A pawn reaching the last rank has four moves with these squares;
          // the board has no piece picker, so it promotes to a queen.
          const played =
            moves.find((move) => move.uci === from + to) ??
            moves.find((move) => move.uci === from + to + 'q') ??
            moves.find((move) => move.uci.startsWith(from + to))
          if (played) onmove(played)
        },
      },
    }
  }

  $effect(() => {
    const next = config()
    if (ground) {
      ground.set(next)
    } else {
      ground = Chessground(element, next)
    }
  })

  $effect(() => () => ground?.destroy())
</script>

<div class="board" bind:this={element}></div>

<style>
  .board {
    width: 100%;
    aspect-ratio: 1;
  }
</style>
