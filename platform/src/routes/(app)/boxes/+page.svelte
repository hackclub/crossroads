<script>
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { date, effectShort } from '#lib/format.js';

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

	<section class="stack">
		<h2>Your special stamps ({stamps.length})</h2>
		{#if stamps.length}
			<!-- A stamp album: each stamp you own, with what it does. -->
			<ul class="album">
				{#each stamps as s}
					<li class="card owned" class:used={s.submission_id && !s.project_id}>
						{#if s.type.image_url}<img src={s.type.image_url} alt="" />{:else}<span class="blank" aria-hidden="true"></span>{/if}
						<strong>{s.type.name}</strong>
						<span class="small muted">{s.type.rarity} · {effectShort(s.type.effect)}</span>
						{#if s.project_id}<a class="small" href="/projects/{s.project_id}">On a project</a>
						{:else if s.submission_id}<span class="small">Used on a ship</span>
						{:else}<span class="small">In your stamp box</span>{/if}
					</li>
				{/each}
			</ul>
		{:else}
			<p class="muted small">None yet.</p>
		{/if}
		{#if opened.length}<p class="small muted">Opened boxes: {opened.map((b) => `${typeName(b.box_type_id)} → ${stampName(b.special_stamp_id)}`).join(' · ')}</p>{/if}
	</section>
</div>

<style>
	.drop { flex-direction: row; align-items: center; gap: 18px; }
	.drop-stamp { height: 120px; width: auto; flex: none; rotate: -3deg; filter: drop-shadow(0 2px 6px rgb(0 0 0 / 0.5)); }
	.named { display: inline-flex; align-items: center; gap: 10px; }
	.thumb { height: 40px; width: auto; filter: drop-shadow(0 1px 2px rgb(0 0 0 / 0.5)); }
	.thumb-blank { width: 34px; height: 40px; border: 1px dashed var(--head); border-radius: 3px; }
	.album {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
		gap: 16px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.owned { align-items: center; gap: 6px; padding: 18px 16px; text-align: center; }
	.owned img, .owned .blank { height: 150px; width: auto; margin-bottom: 6px; filter: drop-shadow(0 2px 6px rgb(0 0 0 / 0.5)); }
	.owned .blank { width: 120px; border: 2px dashed var(--head); border-radius: 6px; filter: none; }
	.owned.used img { filter: grayscale(0.7) brightness(0.8) drop-shadow(0 2px 6px rgb(0 0 0 / 0.5)); }
</style>
