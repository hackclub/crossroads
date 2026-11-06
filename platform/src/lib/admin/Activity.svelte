<script>
	// Daily active participants: people whose projects got Hackatime time on a
	// UTC day, on the Hackatime projects linked to them (GET /admin/activity).
	// The backend refreshes it every half hour, so today fills in as it goes.
	import { onMount } from 'svelte';
	import { get } from '../api.js';
	import { hours, date } from '../format.js';

	const THRESHOLDS = [
		[0, 'Any time'],
		[15, 'At least 15 minutes'],
		[60, 'At least an hour']
	];
	let minMinutes = $state(0);
	let data = $state(null);
	let msg = $state(null);

	async function load() {
		msg = null;
		try {
			data = await get(`/admin/activity?days=30&min_minutes=${minMinutes}`);
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	}
	onMount(load);

	const today = $derived(data?.days.at(-1));
	const peak = $derived(Math.max(1, ...(data?.days.map((d) => d.active_participants) ?? [])));
	const label = (day) => new Date(day + 'T00:00:00Z').toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
</script>

<div class="stack">
	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}
	<label class="field threshold">
		<span>Counts as active</span>
		<select bind:value={minMinutes} onchange={load}>
			{#each THRESHOLDS as [minutes, text]}<option value={minutes}>{text}</option>{/each}
		</select>
	</label>
	{#if data}
		<div class="tiles">
			<div class="card tile"><span class="big">{today.active_participants}</span><span class="small muted">active today</span></div>
			<div class="card tile"><span class="big">{data.last_7_days}</span><span class="small muted">in the last 7 days</span></div>
			<div class="card tile"><span class="big">{data.last_30_days}</span><span class="small muted">in the last 30 days</span></div>
		</div>
		<div class="card">
			<h2>Active participants per day</h2>
			<!-- The table below has the same numbers for screen readers. -->
			<div class="chart" aria-hidden="true">
				{#each data.days as d}
					<div
						class="bar"
						class:empty={d.active_participants === 0}
						style:--h={d.active_participants / peak}
						title="{label(d.day)}: {d.active_participants} active, {d.active_projects} projects, {hours(d.seconds)}"
					></div>
				{/each}
			</div>
			<div class="axis small muted" aria-hidden="true"><span>{label(data.days[0].day)}</span><span>Today</span></div>
		</div>
		<div class="card table-wrap">
			<table>
				<thead><tr><th>Day (UTC)</th><th>Active participants</th><th>Projects worked on</th><th>Time</th></tr></thead>
				<tbody>
					{#each data.days.toReversed() as d}
						<tr><td>{label(d.day)}</td><td>{d.active_participants}</td><td>{d.active_projects}</td><td>{hours(d.seconds)}</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
		<p class="small muted">
			{data.updated_at ? `Updated ${date(data.updated_at)}.` : 'No project time recorded yet.'}
			Counts participants whose linked Hackatime projects got time that day, refreshed every 30 minutes. Reviewers and admins
			aren't counted, and nor is time on Hackatime projects nobody has linked yet.
		</p>
	{/if}
</div>

<style>
	.threshold {
		max-width: 320px;
	}

	.tiles {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: var(--gap);
	}

	.tile {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.chart {
		display: flex;
		align-items: flex-end;
		gap: 4px;
		height: 160px;
		margin-top: 12px;
	}

	.bar {
		flex: 1;
		min-height: 2px;
		height: calc(var(--h) * 100%);
		background: var(--light);
		border-radius: 3px 3px 0 0;
	}

	.bar.empty {
		opacity: 0.3;
	}

	.axis {
		display: flex;
		justify-content: space-between;
		margin-top: 6px;
	}

	@media (max-width: 700px) {
		.tiles {
			grid-template-columns: 1fr;
		}
	}
</style>
