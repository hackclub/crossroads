<script>
	import { onMount } from 'svelte';
	import { get } from '#lib/api.js';
	import { hours, date } from '#lib/format.js';

	let queue = $state(null);
	let error = $state('');
	onMount(async () => {
		try {
			queue = await get('/review/queue');
		} catch (e) {
			error = e.message;
		}
	});
</script>

<div class="page-head">
	<div class="stack-sm">
		<h1>Review queue</h1>
		<p class="muted">Priority-review stamps first, then oldest first.</p>
	</div>
</div>

{#if error}<p class="callout warn">{error}</p>{/if}
{#if queue?.length}
	<div class="card table-wrap">
		<table>
			<thead><tr><th>Project</th><th>By</th><th>Claims</th><th>Hackatime projects</th><th>Submitted</th><th></th></tr></thead>
			<tbody>
				{#each queue as q}
					<tr>
						<td>{#if q.priority}<span class="badge warn">Priority</span> {/if}<strong>{q.project_name}</strong></td>
						<td>{q.user_name}</td>
						<td>{hours(q.new_seconds)}</td>
						<td class="small">{q.hackatime_projects.join(', ')}</td>
						<td class="small">{date(q.submitted_at)}</td>
						<td><a class="btn small primary" href="/review/{q.id}">Review</a></td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{:else if queue}
	<div class="card dashed"><p class="muted">Nothing waiting for review.</p></div>
{/if}
