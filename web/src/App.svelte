<script lang="ts">
  import Board from './lib/Board.svelte'
  import {
    fetchDatabase,
    fetchNode,
    searchPlayers,
    type Color,
    type Database,
    type Move,
    type Player,
    type TreeNode,
  } from './lib/api'

  interface Step {
    san: string
    uci: string
    position: string
  }

  const halfLives = [
    { years: 1, label: '1 year' },
    { years: 2, label: '2 years' },
    { years: 5, label: '5 years' },
    { years: 0, label: 'Off' },
  ]

  let database = $state<Database>()
  let query = $state('')
  let suggestions = $state<Player[]>([])
  let opponent = $state<Player>()
  let myColor = $state<Color>('white')
  let halfLifeYears = $state(2)
  let line = $state<Step[]>([])
  let node = $state<TreeNode>()
  let error = $state('')

  const current = $derived(line.at(-1))
  const opponentColor = $derived<Color>(myColor === 'white' ? 'black' : 'white')

  fetchDatabase().then(
    (loaded) => (database = loaded),
    (reason) => (error = String(reason.message ?? reason)),
  )

  // Player suggestions while typing. A stale answer must not overwrite a newer one.
  $effect(() => {
    const text = query.trim()
    if (opponent || text.length < 2) {
      suggestions = []
      return
    }
    let stale = false
    searchPlayers(text).then(
      (players) => !stale && (suggestions = players),
      (reason) => !stale && (error = reason.message),
    )
    return () => (stale = true)
  })

  // The node for the current position, refetched when any input changes.
  $effect(() => {
    if (!opponent) {
      node = undefined
      return
    }
    let stale = false
    fetchNode(opponent.name, myColor, halfLifeYears, current?.position).then(
      (loaded) => {
        if (stale) return
        node = loaded
        error = ''
      },
      (reason) => !stale && (error = reason.message),
    )
    return () => (stale = true)
  })

  function choose(player: Player) {
    opponent = player
    query = player.name
    line = []
  }

  function clearOpponent() {
    opponent = undefined
    query = ''
    line = []
  }

  function setColor(color: Color) {
    myColor = color
    line = []
  }

  function play(move: Move) {
    line = [...line, { san: move.san, uci: move.uci, position: move.position }]
  }

  function goTo(length: number) {
    line = line.slice(0, length)
  }

  /** "1. e4" for White's moves, "1... c5" for Black's. */
  function numbered(ply: number, san: string, always = false): string {
    const number = Math.floor(ply / 2) + 1
    if (ply % 2 === 0) return `${number}. ${san}`
    return always ? `${number}... ${san}` : san
  }

  const percent = (fraction: number) => `${Math.round(fraction * 100)}%`
  const month = (date?: string) => date?.slice(0, 7) ?? ''
</script>

<header>
  <h1>opposition</h1>
  {#if database}
    <p class="muted">
      {database.games.toLocaleString()} games loaded{#if database.skipped.length}, {database.skipped
          .length} unreadable and skipped{/if}
    </p>
  {/if}
</header>

<section class="setup">
  <div class="field search">
    <label for="opponent">Opponent</label>
    <div class="search-box">
      <input
        id="opponent"
        type="search"
        placeholder="Type a name from the database"
        autocomplete="off"
        bind:value={query}
        oninput={() => opponent && (opponent = undefined)}
      />
      {#if opponent}
        <button class="link" onclick={clearOpponent}>Change</button>
      {/if}
    </div>
    {#if suggestions.length}
      <ul class="suggestions">
        {#each suggestions as player (player.name)}
          <li>
            <button onclick={() => choose(player)}>
              <span>{player.name}</span>
              <span class="muted">{player.games} games</span>
            </button>
          </li>
        {/each}
      </ul>
    {:else if !opponent && query.trim().length >= 2}
      <p class="muted hint">No player with that name.</p>
    {/if}
  </div>

  <div class="field">
    <span class="label">You play</span>
    <div class="toggle">
      <button class:active={myColor === 'white'} onclick={() => setColor('white')}>White</button>
      <button class:active={myColor === 'black'} onclick={() => setColor('black')}>Black</button>
    </div>
  </div>

  <div class="field">
    <label for="half-life">Older games count half after</label>
    <select id="half-life" bind:value={halfLifeYears}>
      {#each halfLives as option (option.years)}
        <option value={option.years}>{option.label}</option>
      {/each}
    </select>
  </div>
</section>

{#if error}
  <p class="error" role="alert">{error}</p>
{/if}

{#if opponent && node}
  <main>
    <div class="board-column">
      <Board
        position={node.position}
        orientation={myColor}
        sideToMove={node.sideToMove}
        moves={node.moves}
        lastMove={current?.uci}
        onmove={play}
      />
    </div>

    <div class="tree-column">
      <nav class="line" aria-label="Moves played">
        <button class="link" onclick={() => goTo(0)} disabled={line.length === 0}>Start</button>
        {#each line as step, ply (ply)}
          <button class="link" class:current={ply === line.length - 1} onclick={() => goTo(ply + 1)}>
            {numbered(ply, step.san, ply === 0)}
          </button>
        {/each}
      </nav>

      <h2>
        {#if node.mover === 'opponent'}
          What {opponent.name} plays here
        {:else}
          Your move: what {opponent.name} has faced here
        {/if}
      </h2>
      <p class="muted">
        {node.games}
        {node.games === 1 ? 'game' : 'games'} with {opponent.name} as {opponentColor} reached this position.
      </p>

      {#if node.moves.length}
        <table>
          <thead>
            <tr>
              <th>Move</th>
              <th class="number">Games</th>
              <th>Frequency</th>
              <th>{opponent.name.split(',')[0]} scored</th>
              <th>Last</th>
            </tr>
          </thead>
          <tbody>
            {#each node.moves as move (move.uci)}
              <tr onclick={() => play(move)}>
                <td>
                  <button class="move">{numbered(line.length, move.san, true)}</button>
                </td>
                <td class="number">{move.games}</td>
                <td>
                  <div class="frequency" title="Share of games, older games discounted">
                    <div class="bar"><div class="fill" style:width={percent(move.share)}></div></div>
                    <span>{percent(move.share)}</span>
                  </div>
                </td>
                <td>
                  {#if move.score !== undefined}
                    <strong>{percent(move.score)}</strong>
                  {:else}
                    <strong>n/a</strong>
                  {/if}
                  <span class="muted">+{move.wins} ={move.draws} −{move.losses}</span>
                </td>
                <td class="muted">{month(move.lastPlayed)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
        <p class="muted hint">
          Frequency discounts older games; the Games column is the raw count. Lines with only a few
          games say little about habits.
        </p>
      {:else}
        <p>No game in the database continues from here.</p>
      {/if}
    </div>
  </main>
{:else if !error}
  <p class="muted empty">Pick your opponent and your colour to see what they play.</p>
{/if}
