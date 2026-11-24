<script lang="ts">
  import Board from './lib/Board.svelte'
  import Report from './lib/Report.svelte'
  import { month, numbered, pawns, percent } from './lib/format'
  import {
    fetchAnalysis,
    fetchDatabase,
    fetchMoves,
    fetchNode,
    fetchPredictions,
    searchPlayers,
    type Analysis,
    type Color,
    type Database,
    type LegalMoves,
    type Playable,
    type Player,
    type Prediction,
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

  /** Predicted moves are listed down to this probability, at most this many. */
  const leastLikelyShown = 0.02
  const mostPredictionsShown = 8

  const myRatingKey = 'opposition.myRating'

  function rememberedRating(): number | undefined {
    try {
      const stored = Number(localStorage.getItem(myRatingKey))
      return stored >= 100 && stored <= 4000 ? stored : undefined
    } catch {
      return undefined
    }
  }

  let database = $state<Database>()
  let query = $state('')
  let suggestions = $state<Player[]>([])
  let opponent = $state<Player>()
  let myColor = $state<Color>('white')
  let halfLifeYears = $state(2)
  let line = $state<Step[]>([])
  let node = $state<TreeNode>()
  let analysis = $state<Analysis>()
  let view = $state<'explore' | 'report'>('explore')
  // The ratings the report is built for, updated once typing has paused.
  let reportRatings = $state<{ theirs: number; mine: number }>()
  let legal = $state<LegalMoves>()
  let prediction = $state<Prediction>()
  let theirRating = $state(1500)
  let myRating = $state(rememberedRating() ?? 1500)
  let myRatingChosen = rememberedRating() !== undefined
  let error = $state('')

  const current = $derived(line.at(-1))
  const opponentColor = $derived<Color>(myColor === 'white' ? 'black' : 'white')
  const surname = $derived(opponent?.name.split(',')[0] ?? '')
  // Any legal move can be played on the board, not only those in the database.
  const boardMoves = $derived<Playable[]>(
    node && legal?.position === node.position ? legal.moves : (node?.moves ?? []),
  )
  const ratingsUsable = $derived(
    [theirRating, myRating].every((rating) => Number.isFinite(rating) && rating >= 100 && rating <= 4000),
  )
  const predicted = $derived(
    prediction && node && prediction.position === node.position
      ? prediction.moves.filter(
          (move, rank) => rank < mostPredictionsShown && move.probability >= leastLikelyShown,
        )
      : undefined,
  )
  const played = $derived(new Map((node?.moves ?? []).map((move) => [move.uci, move])))
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

  // The legal moves of the current position, so the board can leave the database.
  $effect(() => {
    if (!opponent) {
      legal = undefined
      return
    }
    let stale = false
    fetchMoves(current?.position).then(
      (loaded) => !stale && (legal = loaded),
      (reason) => !stale && (error = reason.message),
    )
    return () => (stale = true)
  })

  // What a player of the opponent's rating usually does where it is their move.
  $effect(() => {
    if (!database?.predictions || !node || node.mover !== 'opponent' || !ratingsUsable) {
      prediction = undefined
      return
    }
    let stale = false
    fetchPredictions(node.position, theirRating, myRating).then(
      (loaded) => !stale && (prediction = loaded),
      (reason) => !stale && (error = reason.message),
    )
    return () => (stale = true)
  })

  // Every change of the ratings would start a new report, so wait until the
  // typing has paused.
  $effect(() => {
    const next = ratingsUsable ? { theirs: theirRating, mine: myRating } : undefined
    const timer = setTimeout(() => (reportRatings = next), 600)
    return () => clearTimeout(timer)
  })

  function choose(player: Player) {
    opponent = player
    query = player.name
    line = []
    theirRating = player.rating ?? 1500
    if (!myRatingChosen) myRating = theirRating
  }

  function chooseMyRating() {
    myRatingChosen = true
    try {
      localStorage.setItem(myRatingKey, String(myRating))
    } catch {
      // Without storage the rating simply is not remembered.
    }
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

  function play(move: Playable) {
    line = [...line, { san: move.san, uci: move.uci, position: move.position }]
  }

  function goTo(length: number) {
    line = line.slice(0, length)
  }

  /** Opens a line from the report on the board. */
  function showLine(steps: Step[]) {
    line = steps
    view = 'explore'
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }
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

  {#if database?.predictions}
    <div class="field" title="Used to predict moves. The model works in Lichess blitz ratings, which tend to run higher than over-the-board Elo.">
      <label for="their-rating">Their rating</label>
      <input id="their-rating" class="rating" type="number" min="100" max="4000" step="50" bind:value={theirRating} />
    </div>
    <div class="field" title="Used to predict moves. The model works in Lichess blitz ratings, which tend to run higher than over-the-board Elo.">
      <label for="my-rating">Your rating</label>
      <input
        id="my-rating"
        class="rating"
        type="number"
        min="100"
        max="4000"
        step="50"
        bind:value={myRating}
        onchange={chooseMyRating}
      />
    </div>
  {/if}
</section>

{#if error}
  <p class="error" role="alert">{error}</p>
{/if}

{#if opponent && node}
  <nav class="views" aria-label="View">
    <button class:active={view === 'explore'} onclick={() => (view = 'explore')}>Explore</button>
    <button class:active={view === 'report'} onclick={() => (view = 'report')}>Prep report</button>
  </nav>
{/if}

{#if opponent && view === 'report' && database}
  <Report
    {opponent}
    {myColor}
    {halfLifeYears}
    theirElo={database.predictions ? reportRatings?.theirs : undefined}
    myElo={database.predictions ? reportRatings?.mine : undefined}
    {database}
    onshow={showLine}
  />
{:else if opponent && node}
  <main>
    <div class="board-column">
      <Board
        position={node.position}
        orientation={myColor}
        sideToMove={node.sideToMove}
        moves={boardMoves}
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

      {#if database?.predictions && node.mover === 'opponent'}
        <h2>What a {theirRating}-rated player usually plays here</h2>
        {#if !ratingsUsable}
          <p class="muted">Enter both ratings (100 to 4000) to see predictions.</p>
        {:else if !predicted || !prediction}
          <p class="muted">Predicting…</p>
        {:else if predicted.length === 0}
          <p>The game is over in this position.</p>
        {:else}
          <div class="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Move</th>
                  <th>Likelihood</th>
                  <th>In {surname}'s games</th>
                </tr>
              </thead>
              <tbody>
                {#each predicted as move (move.uci)}
                  {@const known = played.get(move.uci)}
                  <tr onclick={() => play(move)}>
                    <td>
                      <button class="move">{numbered(line.length, move.san, true)}</button>
                    </td>
                    <td>
                      <div class="frequency">
                        <div class="bar"><div class="fill" style:width={percent(move.probability)}></div></div>
                        <span>{percent(move.probability)}</span>
                      </div>
                    </td>
                    <td class:muted={!known}>
                      {#if known}
                        {known.games} {known.games === 1 ? 'game' : 'games'}
                      {:else}
                        never
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
          <p class="muted hint">
            Predicted by Maia-3 for a {theirRating}-rated player facing {myRating}. It models the
            rating, not {opponent.name} personally, and sees only this position, not the moves before
            it. Typical result at these ratings from here: {surname} wins {percent(prediction.win)},
            draw {percent(prediction.draw)}, loses {percent(prediction.loss)}.
          </p>
        {/if}
      {/if}
    </div>
  </main>

{:else if !error}
  <p class="muted empty">Pick your opponent and your colour to see what they play.</p>
{/if}
