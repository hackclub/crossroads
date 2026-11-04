<script>
	import Note from '#lib/components/Note.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, PROGRAM } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	const rows = $derived(
		[...d.people, { username: d.me.username, name: d.me.name, country: d.me.country, approvedHours: d.hours.approved, ships: d.projects.reduce((n, p) => n + p.ships.length, 0), you: true }]
			.toSorted((a, b) => b.approvedHours - a.approvedHours)
	);
</script>

<div class="page-head">
	<div class="stack-sm">
		<h1>Leaderboard</h1>
		<p class="muted">{rows.filter((r) => r.approvedHours >= PROGRAM.goalHours).length} of {PROGRAM.spots} trip spots claimed</p>
	</div>
</div>

<div class="card table-wrap">
	<table>
		<thead><tr><th>#</th><th>Builder</th><th>Approved hours</th><th>Ships</th><th>Trip</th></tr></thead>
		<tbody>
			{#each rows as r, i}
				<tr class:you={r.you}>
					<td>{i + 1}</td>
					<td><a href="/u/{r.username}">{r.name}</a> <span class="muted small">{r.country}</span>{#if r.you} <span class="small">(you)</span>{/if}</td>
					<td>
						<div class="stack-sm">
							<span>{r.approvedHours}h</span>
							<div class="bar" style="height: 6px; max-width: 160px"><span class="seg approved" style="width: {Math.min((r.approvedHours / PROGRAM.goalHours) * 100, 100)}%"></span></div>
						</div>
					</td>
					<td>{r.ships}</td>
					<td>{r.approvedHours >= PROGRAM.goalHours ? '✓ Qualified' : '—'}</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
<div style="margin-top: 16px">
	<Note from="Flavortown">Consider making the leaderboard opt-in in settings.</Note>
</div>
