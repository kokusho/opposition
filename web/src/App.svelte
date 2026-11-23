<script lang="ts">
  import Board from './lib/Board.svelte'
  import {
    fetchAnalysis,
    fetchDatabase,
    fetchMistakes,
    fetchNode,
    searchPlayers,
    type Analysis,
    type Color,
    type Database,
    type Mistake,
    type Move,
    type Player,
    type Scan,
    type Step,
    type TreeNode,
  } from './lib/api'

  const halfLives = [
    { years: 1, label: '1 year' },
    { years: 2, label: '2 years' },
    { years: 5, label: '5 years' },
    { years: 0, label: 'Off' },
  ]

  /** Centipawns a move must give away to be marked in the table. */
  const notableLoss = 50

  let database = $state<Database>()
  let query = $state('')
  let suggestions = $state<Player[]>([])
  let opponent = $state<Player>()
  let myColor = $state<Color>('white')
  let halfLifeYears = $state(2)
  let line = $state<Step[]>([])
  let node = $state<TreeNode>()
  let analysis = $state<Analysis>()
  let scan = $state<Scan>()
  let scanning = $state(false)
  let error = $state('')

  const current = $derived(line.at(-1))
  const opponentColor = $derived<Color>(myColor === 'white' ? 'black' : 'white')
  const surname = $derived(opponent?.name.split(',')[0] ?? '')
  const analysed = $derived(
    analysis && node && analysis.position === node.position
      ? new Map(analysis.moves.map((move) => [move.uci, move]))
      : undefined,
  )

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

  // The node for the current position, refetched when any input changes. The
  // engine's view of it follows separately because it takes a few seconds.
  $effect(() => {
    if (!opponent) {
      node = undefined
      analysis = undefined
      return
    }
    const withEngine = database?.engine
    const args = [opponent.name, myColor, halfLifeYears, current?.position] as const
    let stale = false
    fetchNode(...args).then(
      (loaded) => {
        if (stale) return
        node = loaded
        error = ''
      },
      (reason) => !stale && (error = reason.message),
    )
    if (withEngine) {
      fetchAnalysis(...args).then(
        (loaded) => !stale && (analysis = loaded),
        (reason) => !stale && (error = reason.message),
      )
    }
    return () => (stale = true)
  })

  // A scan belongs to one opponent, colour and half-life. Changing any of
  // them drops it; while one is running, poll for its progress.
  $effect(() => {
    void [opponent, myColor, halfLifeYears]
    scan = undefined
    scanning = false
  })

  $effect(() => {
    if (!scanning || !opponent) return
    const args = [opponent.name, myColor, halfLifeYears] as const
    let stale = false
    let timer: ReturnType<typeof setTimeout>
    const poll = () =>
      fetchMistakes(...args).then(
        (state) => {
          if (stale) return
          scan = state
          if (state.status === 'running') {
            timer = setTimeout(poll, 700)
          } else {
            scanning = false
          }
        },
        (reason) => {
          if (stale) return
          error = reason.message
          scanning = false
        },
      )
    poll()
    return () => {
      stale = true
      clearTimeout(timer)
    }
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

  function show(mistake: Mistake) {
    line = mistake.line
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  /** "1. e4" for White's moves, "1... c5" for Black's. */
  function numbered(ply: number, san: string, always = false): string {
    const number = Math.floor(ply / 2) + 1
    if (ply % 2 === 0) return `${number}. ${san}`
    return always ? `${number}... ${san}` : san
  }

  /** A run of moves as text, starting at the given half-move. */
  function moveText(sans: string[], firstPly: number): string {
    return sans.map((san, index) => numbered(firstPly + index, san, index === 0)).join(' ')
  }

  const percent = (fraction: number) => `${Math.round(fraction * 100)}%`
  const month = (date?: string) => date?.slice(0, 7) ?? ''
  const pawns = (centipawns: number) => (centipawns / 100).toFixed(1)
</script>

<header>
  <h1>opposition</h1>
  {#if database}
    <p class="muted">
      {database.games.toLocaleString()} games loaded{#if database.skipped.length}, {database.skipped
          .length} unreadable and skipped{/if}{#if !database.engine}. No chess engine: evaluations
        are off{/if}
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
      {#if database?.engine}
        <p class="engine-line">
          {#if analysed && analysis}
            <strong class="eval" class:good={analysis.eval.centipawns > 30} class:bad={analysis.eval.centipawns < -30}>
              {analysis.eval.text}
            </strong>
            <span class="muted">
              for you{#if analysis.best}; engine plays {numbered(line.length, analysis.best, true)}{/if}
              (depth {analysis.eval.depth})
            </span>
          {:else}
            <span class="muted">Engine is thinking…</span>
          {/if}
        </p>
      {/if}
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
        <div class="table-scroll">
        <table>
          <thead>
            <tr>
              <th>Move</th>
              <th class="number">Games</th>
              <th>Frequency</th>
              <th>{surname} scored</th>
              {#if database?.engine}
                <th title="Engine evaluation after the move, from your point of view">Engine</th>
              {/if}
              <th>Last</th>
            </tr>
          </thead>
          <tbody>
            {#each node.moves as move (move.uci)}
              {@const verdict = analysed?.get(move.uci)}
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
                {#if database?.engine}
                  <td>
                    {#if verdict}
                      <strong class="eval" class:good={verdict.eval.centipawns > 30} class:bad={verdict.eval.centipawns < -30}>
                        {verdict.eval.text}
                      </strong>
                      {#if verdict.loss >= notableLoss}
                        <span
                          class="badge"
                          class:gift={node.mover === 'opponent'}
                          title={node.mover === 'opponent'
                            ? 'How much this move gives away compared with the best move'
                            : 'How much worse this is than the best move'}
                        >
                          {node.mover === 'opponent' ? 'gives away' : 'costs'}
                          {pawns(verdict.loss)}
                        </span>
                      {/if}
                    {:else}
                      <span class="muted">…</span>
                    {/if}
                  </td>
                {/if}
                <td class="muted">{month(move.lastPlayed)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
        </div>
        <p class="muted hint">
          Frequency discounts older games; the Games column is the raw count. Lines with only a few
          games say little about habits.
        </p>
      {:else}
        <p>No game in the database continues from here.</p>
      {/if}
    </div>
  </main>

  {#if database?.engine}
    <section class="mistakes">
      <div class="mistakes-head">
        <h2>Recurring mistakes</h2>
        {#if !scan && !scanning}
          <button class="primary" onclick={() => (scanning = true)}>Scan {surname}'s habits</button>
        {/if}
      </div>

      {#if !scan && !scanning}
        <p class="muted">
          Checks every move {opponent.name} has played at least twice as {opponentColor} with the
          engine and lists the ones that give away half a pawn or more.
        </p>
      {:else if !scan || scan.status === 'running'}
        <p class="muted">
          Checked {scan?.done ?? 0} of {scan?.total ?? '…'} habitual moves
        </p>
        <progress value={scan?.done ?? 0} max={scan?.total || 1}></progress>
      {:else if scan.status === 'failed'}
        <p class="error" role="alert">The scan failed: {scan.error}</p>
      {:else if scan.items.length === 0}
        <p>
          None found among {scan.total} habitual moves. Either {opponent.name} knows these lines, or
          the database has too few of their games to show a pattern.
        </p>
      {:else}
        <p class="muted">
          {scan.items.length} of {scan.total} habitual moves give away half a pawn or more. Most
          reliable and most costly first.
        </p>
        <ol>
          {#each scan.items as mistake (mistake.position + mistake.move.uci)}
            <li>
              <button class="mistake" onclick={() => show(mistake)}>
                <span class="mistake-line">
                  {moveText(mistake.line.map((step) => step.san), 0)}
                  <strong>{numbered(mistake.line.length, mistake.move.san, mistake.line.length === 0)}?</strong>
                </span>
                <span class="mistake-facts">
                  <span class="badge gift">gives away {pawns(mistake.loss)}</span>
                  <span>
                    played in {mistake.move.games} of {mistake.nodeGames} games ({percent(mistake.move.share)})
                  </span>
                  {#if mistake.best}
                    <span class="muted">
                      engine prefers {numbered(mistake.line.length, mistake.best, true)}
                    </span>
                  {/if}
                </span>
                {#if mistake.refutation.length}
                  <span class="mistake-reply">
                    You continue {moveText(mistake.refutation, mistake.line.length + 1)}
                    <strong class="eval good">{mistake.eval.text}</strong>
                  </span>
                {/if}
              </button>
            </li>
          {/each}
        </ol>
      {/if}
    </section>
  {/if}
{:else if !error}
  <p class="muted empty">Pick your opponent and your colour to see what they play.</p>
{/if}
