<script>
	// The last car of the train (Boxes): every special stamp you own, laid out
	// on wood like the stamp box. Stamps stuck on a project link to it.
	import { get } from '../api.js';
	import { rail } from '../rail.svelte.js';
	import { effectShort } from '../format.js';
	import wood from '../assets/rail/wood.webp';

	let stamps = $state(null);
	let error = $state('');

	$effect(() => {
		rail.stampsVersion; // reload when the Boxes page opens a box
		get('/special-stamps')
			.then((s) => (stamps = s))
			.catch((e) => (error = e.message));
	});

	const tilt = (id) => ((id * 37) % 17) - 8;
</script>

<div class="stamp-car" style="--wood: url({wood})">
	<div class="head">
		<h2>Your stamps{stamps ? ` (${stamps.length})` : ''}</h2>
		<span class="small">Stick them on your projects from a project's page.</span>
	</div>
	<div class="tray">
		{#if error}
			<p class="small">{error}</p>
		{:else if stamps === null}
			<p class="small">Loading…</p>
		{:else if !stamps.length}
			<p class="small">No stamps yet. Open boxes to collect some.</p>
		{:else}
			<ul>
				{#each stamps as s (s.id)}
					<li style="rotate: {tilt(s.id)}deg">
						{#if s.type.image_url}<img src={s.type.image_url} alt="" />{:else}<span class="blank"></span>{/if}
						<span class="name">{s.type.name}</span>
						{#if s.project_id}
							<a class="where" href="/projects/{s.project_id}">On a project</a>
						{:else if s.submission_id}
							<span class="where">Used on a ship</span>
						{:else}
							<span class="where">{effectShort(s.type.effect)} · in your box</span>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</div>

<style>
	.stamp-car { display: flex; flex-direction: column; gap: 12px; height: 100%; padding: 20px 24px 24px; }
	.head { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 4px 16px; }
	.head h2 { font-size: clamp(32px, 3vw, 44px); }
	/* The stamp box's tray: wood under the light leak, inset. */
	.tray {
		flex: 1;
		min-height: 150px;
		padding: 16px 18px;
		overflow-x: auto;
		border-radius: 6px;
		background: linear-gradient(rgb(61 46 24 / 0.8), rgb(61 46 24 / 0.8)), var(--wood) center / cover;
		box-shadow: inset 0 0 0 999px rgb(0 0 0 / 0.2), inset 0 2px 8px rgb(0 0 0 / 0.5);
	}
	ul { display: flex; gap: 22px; margin: 0; padding: 4px 4px 0; list-style: none; }
	li { flex: none; display: flex; flex-direction: column; align-items: center; gap: 2px; width: 120px; text-align: center; }
	li img, .blank { height: 96px; width: auto; max-width: 120px; filter: drop-shadow(0 3px 6px rgb(0 0 0 / 0.55)); }
	.blank { width: 76px; border: 2px dashed rgb(255 255 255 / 0.5); filter: none; }
	.name { margin-top: 6px; font-size: 20px; }
	.where { font-size: 17px; color: rgb(255 255 255 / 0.8); }
	a.where { color: var(--text); }
	@media (max-width: 820px) { .stamp-car { padding-inline: 16px; } }
</style>
