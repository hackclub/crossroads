<script>
	// Hackatime's side of a ship: its numbers, each Hackatime project, and
	// time per day. Daily totals only: reviewers never see heartbeats.
	import { hours, utcDay } from '#lib/format.js';
	import { dayColumns } from './days.js';
	import DayChart from './DayChart.svelte';

	let { hackatime, hackatimeError, submission, earlierSeconds, days, windowStart, reship, checks } = $props();
	const since = $derived(reship ? 'since the last approved ship' : 'since Crossroads started');
	const columns = $derived(dayColumns(days, windowStart));
	const busiest = $derived(columns.reduce((b, c) => (c.seconds > (b?.seconds ?? 0) ? c : b), null));
	const projectMax = $derived(Math.max(1, ...(hackatime?.projects ?? []).map((p) => p.seconds)));
	// The checks that are about Hackatime, with how they came out.
	const hackatimeChecks = $derived((checks ?? []).filter((c) => ['hackatime_flagged', 'hackatime_dropped', 'long_day', 'large_ship'].includes(c.id)));
</script>

{#if hackatimeError}<p class="callout warn small">Couldn't ask Hackatime: {hackatimeError}</p>{/if}
<div class="tab-grid">
	<div class="left stack">
		<div class="tiles">
			<div class="tile"><span class="label">Claimed by this ship</span><strong>{hours(submission.new_seconds)}</strong></div>
			<div class="tile"><span class="label">Tracked when shipped</span><strong>{hours(submission.hackatime_seconds)}</strong></div>
			<div class="tile"><span class="label">Tracked now</span><strong>{hackatime ? hours(hackatime.seconds) : '–'}</strong></div>
			<div class="tile"><span class="label">Paid on earlier ships</span><strong>{hours(earlierSeconds)}</strong></div>
		</div>
		<ul class="status small">
			{#each hackatimeChecks as c (c.id)}
				<li class={c.status}><span class="dot" aria-hidden="true"></span><span>{c.name}</span><span class="muted">{c.status === 'pass' ? 'yes' : c.status === 'fail' ? 'no' : "couldn't check"}</span></li>
			{/each}
		</ul>
		{#if hackatime?.projects.length}
			<div class="stack-sm">
				<h3 class="small">Hackatime projects on this ship</h3>
				<ul class="projects small">
					{#each hackatime.projects as p (p.name)}
						<li>
							<span class="name">{p.name}</span>
							<span class="track"><span class="fill" style="width: {(p.seconds / projectMax) * 100}%"></span></span>
							<span class="num">{hours(p.seconds)}</span>
						</li>
					{/each}
				</ul>
				<p class="small muted">Time {since}, as of now. Time that counted toward two projects is counted once in the total.</p>
			</div>
		{/if}
	</div>
	<div class="right stack">
		{#if columns.length}
			<DayChart {columns} value={(c) => c.seconds} label="Hours per day (UTC), {since}" height={120} format={hours}
				summary="{hours(columns.reduce((s, c) => s + c.seconds, 0))} in all{busiest ? ` · busiest ${utcDay(busiest.day)} (${hours(busiest.seconds)})` : ''}" />
			<details class="small">
				<summary>As a table</summary>
				<table>
					<thead><tr><th>Day (UTC)</th><th>Hackatime</th><th>Commits</th></tr></thead>
					<tbody>
						{#each columns.filter((c) => c.seconds || c.commits) as c (c.day)}
							<tr><td>{utcDay(c.day)}</td><td>{hours(c.seconds)}</td><td>{c.commits}</td></tr>
						{/each}
					</tbody>
				</table>
			</details>
		{:else}
			<p class="small muted">No daily time recorded yet.</p>
		{/if}
	</div>
</div>

<style>
	.tab-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px 28px; }
	@media (min-width: 1300px) { .tab-grid { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); align-items: start; } }
	.tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); gap: 8px; }
	.tile { display: flex; flex-direction: column; padding: 6px 12px; border-radius: 10px; background: var(--navy); }
	.tile .label { font-size: 15px; color: var(--muted); }
	.tile strong { font-size: 22px; font-weight: 400; font-variant-numeric: tabular-nums; }
	.status, .projects { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 4px; }
	.status li { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 10px; }
	.dot { width: 9px; height: 9px; border-radius: 50%; background: var(--muted); }
	.status li.pass .dot { background: var(--good); }
	.status li.fail .dot { background: var(--danger); }
	.projects li { display: grid; grid-template-columns: minmax(0, 12em) minmax(0, 1fr) 4em; align-items: center; gap: 10px; }
	.name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.track { height: 8px; border-radius: 4px; background: var(--rv-sunk, rgb(0 0 0 / 0.3)); overflow: hidden; }
	.fill { display: block; height: 100%; background: var(--info); border-radius: 4px; }
	.num { text-align: right; font-variant-numeric: tabular-nums; }
	details summary { cursor: pointer; color: var(--muted); font-size: 16px; }
	table { width: 100%; border-collapse: collapse; margin-top: 6px; font-size: 16px; }
	th, td { text-align: left; padding: 4px 8px; border-bottom: 1px solid var(--deep); font-variant-numeric: tabular-nums; }
</style>
