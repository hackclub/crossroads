<script>
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { date, effectShort } from '#lib/format.js';
	import { rail } from '#lib/rail.svelte.js';

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
			rail.stampsVersion++; // the stamp car in the train shows the new stamp
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
			<p class="muted">You get a box each time your approved hours pass a milestone. Each box drops one special stamp into your stamp box, and you stick stamps onto your projects from a project's page. Boxes can't be bought.</p>
		</div>
	</div>

	{#if error}<p class="callout warn">{error}</p>{/if}
	{#if dropped}
		<section class="callout good drop" role="status">
			{#if dropped.type.image_url}<img class="drop-stamp" src={dropped.type.image_url} alt="" />{/if}
			<div class="stack-sm">
				<strong>You got: {dropped.type.name}</strong>
				<span class="small">{dropped.type.rarity} · {dropped.type.description || 'A collectible stamp.'} It's in your stamp box: drag it onto one of your projects from the project's page.</span>
			</div>
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
								<td><span class="named">{#if o.special_stamp_type.image_url}<img class="thumb" src={o.special_stamp_type.image_url} alt="" />{:else}<span class="thumb-blank" aria-hidden="true"></span>{/if}{o.special_stamp_type.name}</span></td>
								<td>{o.special_stamp_type.rarity}</td>
								<td>{effectShort(o.special_stamp_type.effect)}</td>
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

	<!-- Your stamps are in the last car of the train, above. -->
	{#if opened.length}<p class="small muted">Opened boxes: {opened.map((b) => `${typeName(b.box_type_id)} → ${stampName(b.special_stamp_id)}`).join(' · ')}</p>{/if}
</div>

<style>
	.drop { flex-direction: row; align-items: center; gap: 18px; }
	.drop-stamp { height: 120px; width: auto; flex: none; rotate: -3deg; filter: drop-shadow(0 2px 6px rgb(0 0 0 / 0.5)); }
	.named { display: inline-flex; align-items: center; gap: 10px; }
	.thumb { height: 40px; width: auto; filter: drop-shadow(0 1px 2px rgb(0 0 0 / 0.5)); }
	.thumb-blank { width: 34px; height: 40px; border: 1px dashed var(--head); border-radius: 3px; }
</style>
