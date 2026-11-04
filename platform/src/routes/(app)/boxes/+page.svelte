<script>
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { date } from '#lib/format.js';

	let boxes = $state([]);
	let types = $state([]);
	let stamps = $state([]);
	let error = $state('');
	let dropped = $state(null);
	let opening = $state(null);

	async function load() {
		try {
			[boxes, types, stamps] = await Promise.all([get('/boxes'), get('/box-types'), get('/special-stamps')]);
		} catch (e) {
			error = e.message;
		}
	}
	onMount(load);

	const typeName = (id) => types.find((t) => t.id === id)?.name ?? `Box type ${id}`;
	const unopened = $derived(boxes.filter((b) => !b.opened_at));
	const opened = $derived(boxes.filter((b) => b.opened_at));
	const stampName = (id) => stamps.find((s) => s.id === id)?.type.name ?? '?';

	async function open(box) {
		error = '';
		opening = box.id;
		try {
			dropped = await api('POST', `/boxes/${box.id}/open`);
			await load();
		} catch (e) {
			error = e.message;
		} finally {
			opening = null;
		}
	}
</script>

<div class="stack-lg">
	<div class="page-head">
		<div class="stack-sm">
			<h1>Boxes</h1>
			<p class="muted">You get a box each time your approved hours pass a milestone. Each box drops one special stamp. Boxes can't be bought.</p>
		</div>
	</div>

	{#if error}<p class="callout warn">{error}</p>{/if}
	{#if dropped}
		<section class="callout good">
			<strong>You got: {dropped.type.name}</strong>
			<span class="small">{dropped.type.rarity} · {dropped.type.description || 'A collectible stamp.'}{#if dropped.type.effect !== 'none'} You can use it when you ship a project ({dropped.type.effect.replace('_', ' ')}).{/if}</span>
		</section>
	{/if}

	<section class="stack">
		<h2>Unopened ({unopened.length})</h2>
		{#each unopened as box}
			<div class="card"><div class="row between">
				<span><strong>{typeName(box.box_type_id)}</strong> <span class="small muted">· got {date(box.granted_at)}{box.milestone_id ? '' : ' · gift from an admin'}</span></span>
				<button class="btn primary" disabled={opening === box.id} onclick={() => open(box)}>{opening === box.id ? 'Opening…' : 'Open'}</button>
			</div></div>
		{:else}
			<p class="muted small">No boxes to open. Get more hours approved to reach the next milestone.</p>
		{/each}
	</section>

	<section class="stack">
		<h2>What's in each box</h2>
		<p class="small muted">These are the real chances right now. Stamps that have run out aren't listed.</p>
		{#each types as t}
			<div class="card">
				<strong>{t.name}</strong>
				{#if t.description}<span class="small muted">{t.description}</span>{/if}
				<table>
					<thead><tr><th>Stamp</th><th>Rarity</th><th>Does</th><th>Chance</th></tr></thead>
					<tbody>
						{#each t.odds as o}
							<tr>
								<td>{o.special_stamp_type.name}</td>
								<td>{o.special_stamp_type.rarity}</td>
								<td>{o.special_stamp_type.effect === 'none' ? 'Collectible' : o.special_stamp_type.effect.replace('_', ' ')}</td>
								<td>{(o.chance * 100).toFixed(1)}%</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<p class="muted small">No box types yet.</p>
		{/each}
	</section>

	<section class="stack">
		<h2>Your special stamps ({stamps.length})</h2>
		{#if stamps.length}
			<div class="card table-wrap">
				<table>
					<thead><tr><th>Stamp</th><th>Rarity</th><th>Does</th><th>Got</th><th>Status</th></tr></thead>
					<tbody>
						{#each stamps as s}
							<tr>
								<td>{s.type.name}</td>
								<td>{s.type.rarity}</td>
								<td>{s.type.effect === 'none' ? 'Collectible' : s.type.effect.replace('_', ' ')}</td>
								<td>{date(s.acquired_at)}</td>
								<td>{s.submission_id ? 'Used' : s.type.effect === 'none' ? '–' : 'Ready to use when you ship'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<p class="muted small">None yet.</p>
		{/if}
		{#if opened.length}<p class="small muted">Opened boxes: {opened.map((b) => `${typeName(b.box_type_id)} → ${stampName(b.special_stamp_id)}`).join(' · ')}</p>{/if}
	</section>
</div>
