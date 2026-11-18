<script>
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { date, effectShort } from '#lib/format.js';
	import { rail } from '#lib/rail.svelte.js';
	import BoxOpening from '#lib/BoxOpening.svelte';

	let boxes = $state([]);
	let types = $state([]);
	let stamps = $state([]);
	let projects = $state([]);
	let error = $state('');
	let dropped = $state(null);
	let opening = $state(null);
	// The box being opened in the dialog, and what came out of it.
	let shown = $state(null);
	let reveal = $state(null);
	let dialog;

	async function load() {
		try {
			[boxes, types, stamps, projects] = await Promise.all([get('/boxes'), get('/box-types'), get('/special-stamps'), get('/projects')]);
		} catch (e) {
			error = e.message;
		}
	}
	onMount(load);

	const typeName = (id) => types.find((t) => t.id === id)?.name ?? `Box type ${id}`;
	const unopened = $derived(boxes.filter((b) => !b.opened_at));
	const opened = $derived(boxes.filter((b) => b.opened_at));
	const stampName = (id) => stamps.find((s) => s.id === id)?.type.name ?? '?';
	const projectName = (id) => projects.find((p) => p.id === id)?.name ?? 'a project';
	// Where a box came from, after when they got it.
	const source = (box) => (box.project_id ? ` · came with ${projectName(box.project_id)}` : box.milestone_id ? '' : ' · gift from an admin');

	async function open(box) {
		error = '';
		opening = box.id;
		shown = box;
		reveal = null;
		if (!dialog.open) dialog.showModal();
		// Let the shut box rock for a moment, however fast the server is.
		const wait = new Promise((r) => setTimeout(r, 900));
		try {
			const got = await api('POST', `/boxes/${box.id}/open`);
			await wait;
			dropped = reveal = got;
			rail.stampsVersion++; // the stamp car in the train shows the new stamp
			await load();
		} catch (e) {
			error = e.message;
			dialog.close();
		} finally {
			opening = null;
		}
	}
	const next = $derived(unopened[0]);
</script>

<div class="stack-lg">
	<div class="page-head">
		<div class="stack-sm">
			<h1>Boxes</h1>
			<p class="muted">Hitting certain milestones in approved hours gets you stamp boxes, and so does each project that gets enough hours approved! Stamps can be collected or applied to your projects for special effects when you ship.</p>
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

	<dialog class="opening" bind:this={dialog} aria-labelledby="opening-title" onclose={() => (shown = reveal = null)}>
		{#if shown}
			<h2 id="opening-title">{typeName(shown.box_type_id)}</h2>
			<BoxOpening stamp={reveal} label={typeName(shown.box_type_id)} />
			<div class="got" class:in={reveal} role="status">
				{#if reveal}
					<p class="won">You got <strong>{reveal.type.name}</strong></p>
					<p>{reveal.type.rarity} · {reveal.type.description || 'A collectible stamp.'}</p>
					<p class="small muted">YAY STAMPS! Drag it onto a project from the project's page.</p>
				{/if}
			</div>
			<!-- The buttons come in with the stamp's name, once it's out. -->
			<div class="row actions" class:in={reveal}>
				{#if reveal}
					{#if next}<button class="btn" onclick={() => open(next)}>Open the next box ({unopened.length})</button>{/if}
					<button class="btn primary" onclick={() => dialog.close()}>Done</button>
				{/if}
			</div>
		{/if}
	</dialog>

	<section class="stack">
		<h2>Unopened ({unopened.length})</h2>
		{#each unopened as box}
			<div class="card"><div class="row between">
				<span><strong>{typeName(box.box_type_id)}</strong> <span class="small muted">· got {date(box.granted_at)}{source(box)}</span></span>
				<button class="btn primary" disabled={opening === box.id} onclick={() => open(box)}>{opening === box.id ? 'Opening…' : 'Open'}</button>
			</div></div>
		{:else}
			<p class="muted small">You don't have any boxes to open right now. Time to ship!</p>
		{/each}
	</section>

	<section class="stack">
		<h2>What's in each box</h2>
		<p class="small muted">These are the chance tables for each box, so that you know what you're getting! If a stamp isn't listed, it probably ran out.</p>
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
	.opening {
		width: min(460px, calc(100vw - 32px));
		padding: 24px 28px;
		color: var(--text);
		background: var(--row-bg);
		border: 0;
		border-radius: var(--radius);
		box-shadow: 0 24px 60px rgb(0 0 0 / 0.6);
		font-family: 'Crimson Pro', Georgia, serif;
		text-align: center;
		overflow: hidden;
	}
	.opening::backdrop { background: rgb(0 0 0 / 0.75); }
	.opening h2 { font-size: 32px; min-height: 1.2em; }
	.got { min-height: 6.4em; }
	.got p { margin: 0 0 4px; }
	.got .won { font-size: 26px; }
	.got.in, .actions.in { animation: got-in 500ms ease-out 1900ms both; }
	@keyframes got-in { from { opacity: 0; translate: 0 6px; } }
	.opening .actions { justify-content: center; min-height: 44px; }
	@media (prefers-reduced-motion: reduce) { .got.in, .actions.in { animation: none; } }
	.thumb-blank { width: 34px; height: 40px; border: 1px dashed var(--head); border-radius: 3px; }
</style>
