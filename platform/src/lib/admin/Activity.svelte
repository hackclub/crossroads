<script>
	// Daily activity (GET /admin/activity), by UTC day, participants only:
	// who used the platform and who signed up, from the backend's own records,
	// and whose projects got Hackatime time, on the Hackatime projects linked
	// to them. The backend refreshes project time every half hour, so today
	// fills in as it goes.
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
	const label = (day) => new Date(day + 'T00:00:00Z').toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
</script>

{#snippet tiles(items)}
	<div class="tiles">
		{#each items as [n, text]}<div class="card tile"><span class="big">{n}</span><span class="small muted">{text}</span></div>{/each}
	</div>
{/snippet}

<!-- The table at the bottom has the same numbers for screen readers. -->
{#snippet chart(key, describe)}
	{@const peak = Math.max(1, ...data.days.map((d) => d[key]))}
	<div class="card" aria-hidden="true">
		<div class="chart">
			{#each data.days as d}
				<div class="bar" class:empty={d[key] === 0} style:--h={d[key] / peak} title="{label(d.day)}: {describe(d)}"></div>
			{/each}
		</div>
		<div class="axis small muted"><span>{label(data.days[0].day)}</span><span>Today</span></div>
	</div>
{/snippet}

<div class="stack">
	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}
	{#if data}
		<section class="stack">
			<h2>On the platform</h2>
			{@render tiles([
				[today.visitors, 'today'],
				[data.visitors_last_7_days, 'in the last 7 days'],
				[data.visitors_last_30_days, 'in the last 30 days'],
				[today.sign_ups, 'signed up today']
			])}
			{@render chart('visitors', (d) => `${d.visitors} on the platform, ${d.sign_ups} signed up`)}
		</section>

		<section class="stack">
			<div class="row section-head">
				<h2>Working on projects</h2>
				<label class="field threshold">
					<span>Counts as working</span>
					<select bind:value={minMinutes} onchange={load}>
						{#each THRESHOLDS as [minutes, text]}<option value={minutes}>{text}</option>{/each}
					</select>
				</label>
			</div>
			{@render tiles([
				[today.active_participants, 'today'],
				[data.last_7_days, 'in the last 7 days'],
				[data.last_30_days, 'in the last 30 days']
			])}
			{@render chart('active_participants', (d) => `${d.active_participants} working, ${d.active_projects} projects, ${hours(d.seconds)}`)}
		</section>

		<div class="card table-wrap">
			<table>
				<thead>
					<tr><th>Day (UTC)</th><th>On the platform</th><th>Signed up</th><th>Working on projects</th><th>Projects worked on</th><th>Time</th></tr>
				</thead>
				<tbody>
					{#each data.days.toReversed() as d}
						<tr>
							<td>{label(d.day)}</td><td>{d.visitors}</td><td>{d.sign_ups}</td><td>{d.active_participants}</td><td>{d.active_projects}</td><td>{hours(d.seconds)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<p class="small muted">
			Participants only; reviewers and admins aren't counted. On the platform means signed in and using the site that day.
			Working on projects means their linked Hackatime projects got time that day, refreshed every 30 minutes
			({data.updated_at ? `last ${date(data.updated_at)}` : 'nothing recorded yet'}). Time on Hackatime projects nobody has
			linked yet doesn't count.
		</p>
	{/if}
</div>

<style>
	.section-head {
		justify-content: space-between;
		align-items: flex-end;
	}

	.threshold {
		width: min(100%, 320px);
	}

	.tiles {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
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
</style>
