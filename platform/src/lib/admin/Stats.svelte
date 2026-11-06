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

	const RANGES = [
		[7, '7 days'],
		[30, '30 days'],
		[90, '90 days'],
		[366, 'All time'] // the backend starts at Nov 3 anyway
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
	// "in the last 30 days", or "since Nov 3" when there's less history than that.
	const span = $derived(!data ? '' : data.days.length === range ? `in the last ${range} days` : `since ${utcDay(data.days[0].day)}`);
	const hoursOf = (d) => d.seconds / 3600;
	const formatHours = (h) => (Number.isInteger(h) ? h : h.toFixed(1)) + 'h';
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
			</div>

			<div class="charts">
				<section class="card">
					<div><h3>On the platform</h3><p class="small muted">Participants signed in and using the site</p></div>
					<DailyChart days={data.days} value={(d) => d.visitors} label="on the platform" />
				</section>
				<section class="card">
					<div><h3>Sign-ups</h3><p class="small muted">New participants each day</p></div>
					<DailyChart days={data.days} value={(d) => d.sign_ups} label="signed up" />
				</section>
				<section class="card">
					<div><h3>Participants</h3><p class="small muted">Signed up by the end of each day</p></div>
					<DailyChart days={data.days} value={(d) => d.participants} kind="area" label="participants" />
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
			</div>

			<div class="card table-wrap">
				<table>
					<caption class="small muted">Every day, newest first</caption>
					<thead>
						<tr>
							<th>Day (UTC)</th><th>On the platform</th><th>Signed up</th><th>Participants</th><th>Working on projects</th><th>Projects worked on</th><th>Hours</th>
						</tr>
					</thead>
					<tbody>
						{#each data.days.toReversed() as d}
							<tr>
								<td>{utcDay(d.day)}</td><td>{d.visitors}</td><td>{d.sign_ups}</td><td>{d.participants}</td><td>{d.active_participants}</td><td
									>{d.active_projects}</td
								><td>{hours(d.seconds)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
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

	caption {
		padding: 10px 16px;
		text-align: left;
	}
</style>
