<script>
	// The stats panel (GET /admin/activity): participants only, by UTC day.
	// Platform numbers come from the backend's own records of sign-ups and
	// signed-in visits. Project numbers come from Hackatime time on the
	// Hackatime projects linked to each project, which the backend refreshes
	// every half hour, so today fills in as it goes.
	import { onMount } from 'svelte';
	import { get } from '../api.js';
	import { hours, date, utcDay } from '../format.js';
	import DailyChart from './DailyChart.svelte';
	import HoursSankey from './HoursSankey.svelte';

	const RANGES = [
		[7, '7 days'],
		[30, '30 days'],
		[90, '90 days'],
		[366, 'All time'] // the backend starts at Aug 1 anyway
	];
	const THRESHOLDS = [
		[0, 'any time'],
		[15, 'at least 15 minutes'],
		[60, 'at least an hour']
	];
	let range = $state(30);
	let minMinutes = $state(0);
	let data = $state(null);
	let loading = $state(false);
	let msg = $state(null);

	async function load() {
		loading = true;
		msg = null;
		try {
			data = await get(`/admin/activity?days=${range}&min_minutes=${minMinutes}`);
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		} finally {
			loading = false;
		}
	}
	onMount(load);

	const today = $derived(data?.days.at(-1));
	const sum = (key) => data.days.reduce((total, d) => total + d[key], 0);
	// "in the last 30 days", or "since Aug 1" when there's less history than that.
	const span = $derived(!data ? '' : data.days.length === range ? `in the last ${range} days` : `since ${utcDay(data.days[0].day)}`);
	const hoursOf = (d) => d.seconds / 3600;
	const formatHours = (h) => (Number.isInteger(h) ? h : h.toFixed(1)) + 'h';
	const shippedOf = (d) => d.shipped_seconds / 3600;

	// The hours flow (all time): a node's numbers by key.
	const flow = $derived(data ? Object.fromEntries(data.hours.nodes.map((n) => [n.key, n])) : {});
	const ofLogged = (seconds) => (flow.logged?.seconds ? `${((seconds / flow.logged.seconds) * 100).toFixed(1)}%` : '–');
</script>

<div class="stack">
	<div class="row filters">
		<div class="presets" role="group" aria-label="Range">
			{#each RANGES as [days, text]}
				<button
					class="btn small"
					class:primary={range === days}
					aria-pressed={range === days}
					onclick={() => {
						range = days;
						load();
					}}>{text}</button
				>
			{/each}
		</div>
		<label class="threshold">
			<span class="muted">Working means</span>
			<select bind:value={minMinutes} onchange={load}>
				{#each THRESHOLDS as [minutes, text]}<option value={minutes}>{text}</option>{/each}
			</select>
			<span class="muted">on a day</span>
		</label>
	</div>

	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}

	{#if data}
		<div class="stack" class:loading>
			<div class="tiles">
				<div class="card tile">
					<span class="small muted">Participants</span>
					<span class="big">{today.participants.toLocaleString()}</span>
					<span class="small muted">{sum('sign_ups').toLocaleString()} signed up {span}</span>
				</div>
				<div class="card tile">
					<span class="small muted">On the platform</span>
					<span class="big">{data.range.visitors.toLocaleString()}</span>
					<span class="small muted">{span}; {today.visitors} today</span>
				</div>
				<div class="card tile">
					<span class="small muted">Working on projects</span>
					<span class="big">{data.range.active_participants.toLocaleString()}</span>
					<span class="small muted">{span}; {today.active_participants} today</span>
				</div>
				<div class="card tile">
					<span class="small muted">Hours logged</span>
					<span class="big">{hours(sum('seconds'))}</span>
					<span class="small muted">{span}; {hours(today.seconds)} today</span>
				</div>
				<div class="card tile">
					<span class="small muted">In the Unified YSWS DB</span>
					<span class="big">{hours(flow.unified.seconds)}</span>
					<span class="small muted">all time; {ofLogged(flow.unified.seconds)} of every hour logged, on {flow.unified.ships.toLocaleString()} ship{flow.unified.ships === 1 ? '' : 's'}</span>
				</div>
			</div>

			<section class="card people">
				<div><h3>How far people get</h3><p class="small muted">Participants who ever reached each step. All time, whatever the range above.</p></div>
				<ol class="steps">
					{#each data.people as step, i (step.key)}
						{@const first = data.people[0].people}
						{@const before = i ? data.people[i - 1].people : 0}
						<li>
							<span class="step-label">{step.label}</span>
							<span class="track" title="{step.people} of {first} participants"><span class="fill" style:width="{first ? (step.people / first) * 100 : 0}%"></span></span>
							<span class="count">{step.people.toLocaleString()}</span>
							<span class="rate">{first ? `${Math.round((step.people / first) * 100)}%` : '–'}{#if i}<span class="muted"> · {before ? `${Math.round((step.people / before) * 100)}% of the step before` : '–'}</span>{/if}</span>
						</li>
					{/each}
				</ol>
			</section>

			<section class="card hours">
				<div><h3>Where every hour went</h3><p class="small muted">Every hour logged on a linked project, from shipping to the Unified YSWS DB. All time, whatever the range above.</p></div>
				<HoursSankey nodes={data.hours.nodes} links={data.hours.links} />
			</section>

			<div class="charts">
				<section class="card">
					<div><h3>On the platform</h3><p class="small muted">Participants signed in and using the site</p></div>
					<DailyChart days={data.days} value={(d) => d.visitors} label="on the platform" />
				</section>
				<section class="card">
					<div><h3>Sign-ups</h3><p class="small muted">New participants each day; {today.participants.toLocaleString()} in all by today</p></div>
					<DailyChart days={data.days} value={(d) => d.sign_ups} label="signed up" />
				</section>
				<section class="card">
					<div><h3>Working on projects</h3><p class="small muted">Participants whose linked projects got {THRESHOLDS.find(([m]) => m === minMinutes)[1]}</p></div>
					<DailyChart days={data.days} value={(d) => d.active_participants} label="working on projects" />
				</section>
				<section class="card">
					<div><h3>Projects worked on</h3><p class="small muted">Projects that got any time</p></div>
					<DailyChart days={data.days} value={(d) => d.active_projects} label="projects worked on" />
				</section>
				<section class="card">
					<div><h3>Hours logged</h3><p class="small muted">Hackatime time on linked projects</p></div>
					<DailyChart days={data.days} value={hoursOf} format={formatHours} integer={false} label="logged" />
				</section>
				<section class="card">
					<div><h3>Hours shipped</h3><p class="small muted">Hours claimed by the ships each day, whatever became of them</p></div>
					<DailyChart days={data.days} value={shippedOf} format={formatHours} integer={false} label="shipped" />
				</section>
			</div>

			<details class="card days">
				<summary class="small">Every day as a table, newest first</summary>
				<div class="table-wrap">
				<table>
					<thead>
						<tr>
							<th>Day (UTC)</th><th>On the platform</th><th>Signed up</th><th>Participants</th><th>Working on projects</th><th>Projects worked on</th><th>Hours</th><th>Hours shipped</th>
						</tr>
					</thead>
					<tbody>
						{#each data.days.toReversed() as d}
							<tr>
								<td>{utcDay(d.day)}</td><td>{d.visitors}</td><td>{d.sign_ups}</td><td>{d.participants}</td><td>{d.active_participants}</td><td
									>{d.active_projects}</td
								><td>{hours(d.seconds)}</td><td>{hours(d.shipped_seconds)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
				</div>
			</details>
			<p class="small muted">
				Participants only; reviewers and admins aren't counted. Days are UTC. Project time is refreshed every 30 minutes ({data.updated_at
					? `last ${date(data.updated_at)}`
					: 'nothing recorded yet'}), and time on Hackatime projects nobody has linked yet doesn't count.
			</p>
		</div>
	{/if}
</div>

<style>
	.filters {
		justify-content: space-between;
	}

	.presets {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.threshold {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		font-size: 22px;
	}

	.threshold select {
		width: auto;
		padding: 4px 10px;
		font-size: 22px;
	}

	/* A refetch keeps the old numbers up, dimmed, instead of a flash. */
	.loading {
		opacity: 0.55;
		transition: opacity 0.2s;
	}

	.tiles {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: var(--gap);
	}

	.tile {
		gap: 6px;
	}

	.charts {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(100%, 440px), 1fr));
		gap: var(--gap);
	}

	h3 {
		font-size: 30px;
	}


	.hours { gap: 12px; }

	/* How far people get: one bar per step, all out of everyone signed up,
	   in the daily charts' blue. */
	.people { gap: 12px; }
	.steps { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
	.steps li { display: grid; grid-template-columns: minmax(9em, 14em) minmax(0, 1fr) 4em minmax(12em, 18em); align-items: center; gap: 14px; }
	.step-label { font-size: 18px; }
	.track { height: 12px; border-radius: 4px; background: rgb(255 255 255 / 0.06); overflow: hidden; }
	.fill { display: block; height: 100%; min-width: 2px; border-radius: 0 4px 4px 0; background: #3987e5; }
	.count { text-align: right; font-size: 22px; font-variant-numeric: tabular-nums; }
	.rate { font-size: 16px; font-variant-numeric: tabular-nums; }
	@media (max-width: 720px) {
		.steps li { grid-template-columns: minmax(0, 1fr) auto; gap: 2px 12px; }
		.track { grid-column: 1 / -1; grid-row: 2; }
		.rate { grid-column: 1 / -1; }
	}

	.days summary { cursor: pointer; color: var(--muted); }
	.days[open] summary { margin-bottom: 8px; }
</style>
