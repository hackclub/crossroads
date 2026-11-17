<script>
	// Decided ships, the latest first (GET /review/past). mode:
	//   past     ships the viewer decided; HQ and admins can show everyone's
	//   rejected every rejected ship, soft and hard (HQ and admins)
	//   blocked  hard-rejected projects that can't be shipped again, to let
	//            them (the admin panel)
	import { api, get } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import { hours, date } from '#lib/format.js';
	import { canManageRejections } from '#lib/review.js';

	let { mode = 'past' } = $props();

	const manage = $derived(canManageRejections(session.me));
	let everyone = $state(false); // past: everyone's, not just mine
	let data = $state(null);
	let error = $state('');
	let busy = $state(null); // the project being let back in

	async function load() {
		error = '';
		const q = mode === 'past' ? `scope=${everyone ? 'all' : 'mine'}` : 'status=rejected&scope=all';
		try {
			data = await get(`/review/past?${q}`);
		} catch (e) {
			error = e.message;
		}
	}
	$effect(() => {
		everyone;
		load();
	});

	// A project shows once, by its latest ship, in the blocked list.
	const items = $derived.by(() => {
		if (!data) return [];
		if (mode !== 'blocked') return data.items;
		const seen = new Set();
		return data.items.filter((i) => i.resubmit_blocked && !seen.has(i.project_id) && seen.add(i.project_id));
	});

	async function allow(item) {
		busy = item.project_id;
		error = '';
		try {
			await api('PUT', `/review/projects/${item.project_id}/resubmit-block`, { blocked: false });
			await load();
		} catch (e) {
			error = e.message;
		} finally {
			busy = null;
		}
	}

	const outcome = (i) => (i.status === 'approved' ? 'Approved' : i.resubmit_blocked ? 'Hard rejected' : 'Soft rejected');
	const tone = (i) => (i.status === 'approved' ? 'good' : i.resubmit_blocked ? 'warn' : 'info');
</script>

<div class="stack">
	{#if mode === 'past'}
		<div class="row between">
			<p class="small muted">{everyone ? 'Every decided ship' : 'Ships you made a decision on'}, the latest first.</p>
			{#if manage}<label class="check small"><input type="checkbox" bind:checked={everyone} /> Everyone's</label>{/if}
		</div>
	{:else if mode === 'rejected'}
		<p class="small muted">Soft: they can change it and ship it again. Hard: it can't be shipped again until HQ or an admin allows it. Open one to change it.</p>
	{:else}
		<p class="small muted">Projects a hard reject stopped from being shipped again. Letting one back means its owner can ship it again.</p>
	{/if}
	{#if error}<p class="callout warn small" role="alert">{error}</p>{/if}

	{#if items.length}
		<div class="card table-wrap">
			<table>
				<thead>
					<tr>
						<th>Project</th><th>By</th><th>Outcome</th><th class="num">Claims</th><th class="num">Approved</th><th>Decided</th><th></th>
					</tr>
				</thead>
				<tbody>
					{#each items as i (i.id)}
						<tr>
							<td>
								{i.project_name}
								{#if i.shipped_again}<span class="small muted"> · shipped again since</span>{/if}
							</td>
							<td>{i.user_name}</td>
							<td><span class="badge {tone(i)}">{outcome(i)}</span></td>
							<td class="num">{hours(i.new_seconds)}</td>
							<td class="num">{i.approved_seconds != null ? hours(i.approved_seconds) : '–'}</td>
							<td class="small">{date(i.decided_at)}{i.decided_by ? `, ${i.decided_by}` : ''}</td>
							<td>
								<div class="row end">
									{#if mode === 'blocked'}
										<button class="btn small" disabled={busy === i.project_id} onclick={() => allow(i)}>Let them ship it again</button>
									{/if}
									<a class="btn small ghost" href="/review/{i.id}">Open</a>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else if data}
		<div class="card dashed"><p class="muted">{mode === 'blocked' ? 'No projects are blocked.' : mode === 'rejected' ? 'Nothing rejected.' : 'No decided ships yet.'}</p></div>
	{/if}
</div>

<style>
	.num { text-align: right; font-variant-numeric: tabular-nums; }
	.end { justify-content: flex-end; flex-wrap: nowrap; }
</style>
