<script lang="ts">
  import {
    fetchNarrative,
    fetchReport,
    type Color,
    type Database,
    type Player,
    type ReportState,
    type Step,
  } from './api'
  import { moveText, numbered, pawns, percent } from './format'

  interface Props {
    opponent: Player
    myColor: Color
    halfLifeYears: number
    /** Ratings for the move predictions; undefined lets the server choose. */
    theirElo?: number
    myElo?: number
    database: Database
    /** Called with a line of moves to put on the board. */
    onshow: (steps: Step[]) => void
  }

  let { opponent, myColor, halfLifeYears, theirElo, myElo, database, onshow }: Props = $props()

  let job = $state<ReportState>()
  let error = $state('')
  let narrative = $state('')
  let narrating = $state(false)
  let narrativeError = $state('')

  const report = $derived(job?.report)
  const theirColor = $derived<Color>(myColor === 'white' ? 'black' : 'white')
  const surname = $derived(opponent.name.split(',')[0])
  const sans = (steps: Step[]) => steps.map((step) => step.san)

  // The report belongs to these settings; changing any of them starts over.
  // The statistics come with the first answer, the engine's findings follow
  // while it is polled.
  $effect(() => {
    const args = [opponent.name, myColor, halfLifeYears, theirElo, myElo] as const
    job = undefined
    error = ''
    narrative = ''
    narrativeError = ''
    let stale = false
    let timer: ReturnType<typeof setTimeout>
    const poll = () =>
      fetchReport(...args).then(
        (next) => {
          if (stale) return
          job = next
          if (next.status === 'running') timer = setTimeout(poll, 700)
        },
        (reason) => !stale && (error = reason.message),
      )
    poll()
    return () => {
      stale = true
      clearTimeout(timer)
    }
  })

  async function writeUp() {
    narrating = true
    narrativeError = ''
    try {
      narrative = await fetchNarrative(opponent.name, myColor, halfLifeYears, theirElo, myElo)
    } catch (reason) {
      narrativeError = reason instanceof Error ? reason.message : String(reason)
    } finally {
      narrating = false
    }
  }
</script>

{#if error}
  <p class="error" role="alert">{error}</p>
{:else if !job || !report}
  <p class="muted">Building the report…</p>
{:else}
  <article class="report">
    <header>
      <h2>Preparing against {opponent.name}</h2>
      <p>
        {report.summary.games}
        {report.summary.games === 1 ? 'game' : 'games'} as {theirColor} in the database{#if report.summary.first}, from
          {report.summary.first} to {report.summary.last}{/if}. {surname} scored
        {#if report.summary.score !== undefined}<strong>{percent(report.summary.score)}</strong>{:else}n/a{/if}
        <span class="muted">(+{report.summary.wins} ={report.summary.draws} −{report.summary.losses})</span>.
      </p>
      {#if report.summary.games < 10}
        <p class="warning">
          Only {report.summary.games} games: everything below is thin evidence about {surname}'s habits.
        </p>
      {/if}
    </header>

    {#if job.status === 'running'}
      <section class="progress">
        <p class="muted">
          {#if job.stage === 'traps'}
            Looking for traps: {job.done} of {job.total} positions checked
          {:else if job.stage === 'mistakes'}
            Checking their habitual moves with the engine: {job.done} of {job.total}
          {:else}
            Starting the engine…
          {/if}
        </p>
        <progress value={job.done} max={job.total || 1}></progress>
      </section>
    {:else if job.status === 'failed'}
      <p class="error" role="alert">The engine analysis failed: {job.error}</p>
    {/if}

    <section>
      <h3>Traps</h3>
      <p class="muted">
        Sound moves of yours after which a {report.theirElo}-rated player often goes wrong. The
        percentages are a model's estimate for that rating, not {surname}'s own record.
      </p>
      {#if report.traps === null}
        {#if job.status === 'running'}
          <p class="muted">Not there yet.</p>
        {:else}
          <p>Not analysed. This needs both a chess engine and a move-prediction model.</p>
        {/if}
      {:else if report.traps.length === 0}
        <p>None found in the lines {surname} plays.</p>
      {:else}
        <ol>
          {#each report.traps as trap (trap.position + trap.move.uci)}
            <li>
              <button class="finding" onclick={() => onshow([...trap.line, trap.move])}>
                <span>
                  {#if trap.line.length}{moveText(sans(trap.line), 0)}{/if}
                  <strong>{numbered(trap.line.length, trap.move.san, trap.line.length === 0)}</strong>
                </span>
                <span class="facts">
                  <span class="badge gift">goes wrong {percent(trap.errorChance)}</span>
                  <span><strong class="eval">{trap.eval.text}</strong> for you after the move</span>
                  <span class="muted">
                    {#if trap.facedGames}
                      {surname} faced it {trap.facedGames} {trap.facedGames === 1 ? 'time' : 'times'}
                    {:else}
                      engine suggestion, never played against {surname}
                    {/if}
                  </span>
                </span>
                <span class="replies">
                  {#each trap.replies as reply (reply.uci)}
                    <span>
                      {numbered(trap.line.length + 1, reply.san, true)}
                      <span class="muted">{percent(reply.probability)}</span>
                      {#if reply.loss > 0}
                        loses {pawns(reply.loss)}{#if reply.punish}, answer
                          {numbered(trap.line.length + 2, reply.punish, true)}{/if}
                        <strong class="eval good">{reply.eval.text}</strong>
                      {:else}
                        is fine <span class="eval">{reply.eval.text}</span>
                      {/if}
                    </span>
                  {/each}
                </span>
              </button>
            </li>
          {/each}
        </ol>
      {/if}
    </section>

    <section>
      <h3>Recurring mistakes</h3>
      <p class="muted">
        Moves {surname} played at least twice that give away half a pawn or more according to the engine.
      </p>
      {#if report.mistakes === null}
        {#if job.status === 'running'}
          <p class="muted">Not there yet.</p>
        {:else}
          <p>Not analysed. This needs a chess engine.</p>
        {/if}
      {:else if report.mistakes.length === 0}
        <p>None found. Either {surname} knows these lines, or there are too few games to show a pattern.</p>
      {:else}
        <ol>
          {#each report.mistakes as mistake (mistake.position + mistake.move.uci)}
            <li>
              <button class="finding" onclick={() => onshow(mistake.line)}>
                <span>
                  {moveText(sans(mistake.line), 0)}
                  <strong>{numbered(mistake.line.length, mistake.move.san, mistake.line.length === 0)}?</strong>
                </span>
                <span class="facts">
                  <span class="badge gift">gives away {pawns(mistake.loss)}</span>
                  <span>
                    played in {mistake.move.games} of {mistake.nodeGames} games ({percent(mistake.move.share)})
                  </span>
                  {#if mistake.best}
                    <span class="muted">engine prefers {numbered(mistake.line.length, mistake.best, true)}</span>
                  {/if}
                </span>
                {#if mistake.refutation.length}
                  <span class="replies">
                    <span>
                      You continue {moveText(mistake.refutation, mistake.line.length + 1)}
                      <strong class="eval good">{mistake.eval.text}</strong>
                    </span>
                  </span>
                {/if}
              </button>
            </li>
          {/each}
        </ol>
      {/if}
    </section>

    <section>
      <h3>Moves {surname} scores badly against</h3>
      <p class="muted">
        Faced at least twice with a score of 35% or less, where {surname} did better against something else.
      </p>
      {#if report.weakSpots.length === 0}
        <p>None stand out.</p>
      {:else}
        <ol>
          {#each report.weakSpots as spot (spot.position + spot.move.uci)}
            <li>
              <button
                class="finding row"
                onclick={() => onshow([...spot.line, { san: spot.move.san, uci: spot.move.uci, position: spot.move.position }])}
              >
                <span>
                  {#if spot.line.length}{moveText(sans(spot.line), 0)}{/if}
                  <strong>{numbered(spot.line.length, spot.move.san, spot.line.length === 0)}</strong>
                </span>
                <span class="facts">
                  <span>
                    {surname} scored
                    <strong>{spot.move.score !== undefined ? percent(spot.move.score) : 'n/a'}</strong>
                    in {spot.move.games} games
                  </span>
                  <span class="muted">+{spot.move.wins} ={spot.move.draws} −{spot.move.losses}</span>
                </span>
              </button>
            </li>
          {/each}
        </ol>
      {/if}
    </section>

    <section>
      <h3>Main lines</h3>
      <p class="muted">What {surname}'s games follow, for as long as at least two games stay together.</p>
      {#if report.mainLines.length === 0}
        <p>No line was played more than once.</p>
      {:else}
        <ol>
          {#each report.mainLines as main (sans(main.line).join(' '))}
            <li>
              <button class="finding row" onclick={() => onshow(main.line)}>
                <span>{moveText(sans(main.line), 0)}</span>
                <span class="facts">
                  <span>{main.games} games</span>
                  <span class="muted">
                    {surname} scored {main.score !== undefined ? percent(main.score) : 'n/a'} (+{main.wins} ={main.draws}
                    −{main.losses})
                  </span>
                </span>
              </button>
            </li>
          {/each}
        </ol>
      {/if}
    </section>

    {#if database.narrative}
      <section>
        <h3>Written briefing</h3>
        {#if narrative}
          <p class="narrative">{narrative}</p>
          <p class="muted hint">
            Written by a language model from the findings above. It was told to add nothing, but it can
            still misstate them; the sections above are the source.
          </p>
        {:else}
          <p class="muted">Has a language model put the findings above into a short briefing.</p>
          <button class="primary" onclick={writeUp} disabled={narrating || job.status !== 'done'}>
            {narrating ? 'Writing…' : 'Write it up'}
          </button>
          {#if job.status === 'running'}
            <p class="muted hint">Available once the engine has finished.</p>
          {/if}
          {#if narrativeError}
            <p class="error" role="alert">{narrativeError}</p>
          {/if}
        {/if}
      </section>
    {/if}
  </article>
{/if}

<style>
  .report {
    margin-top: 16px;
    display: grid;
    gap: 14px;
  }

  .report > header,
  .report > section {
    padding: 14px 16px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
  }

  h2,
  h3 {
    margin: 0 0 4px;
  }

  h3 {
    font-size: 1.05rem;
  }

  .warning {
    color: var(--error);
  }

  ol {
    margin: 10px 0 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 8px;
  }

  button.finding {
    display: grid;
    gap: 4px;
    width: 100%;
    padding: 10px 12px;
    text-align: left;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
    overflow-wrap: anywhere;
  }

  button.finding:hover,
  button.finding:focus-visible {
    border-color: var(--accent);
  }

  .facts {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 12px;
    font-size: 0.9rem;
  }

  .facts :global(.badge) {
    margin-left: 0;
  }

  .replies {
    display: grid;
    gap: 2px;
    font-size: 0.9rem;
  }

  .narrative {
    white-space: pre-wrap;
    max-width: 70ch;
  }

  button.primary:disabled {
    opacity: 0.5;
    cursor: default;
  }
</style>
