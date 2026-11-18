<script>
	// What's waiting for someone takes over Home until they've opened it, so
	// it's the first thing they see: a project's stamp box (from a per-project
	// milestone) once the ship that earned it is signed off, and a stamp an
	// admin gave them, which comes in a box too. "Later" puts it all off until
	// they come back in a new tab; boxes wait on the Boxes page meanwhile, and
	// stamps in the stamp box.
	import { api } from './api.js';
	import { rail } from './rail.svelte.js';
	import BoxOpening from './BoxOpening.svelte';

	// boxes: GET /boxes. stamps: GET /special-stamps. projects: GET /projects,
	// for the names. onopened runs after each one opens, to reload them.
	let { boxes, stamps, projects, onopened } = $props();

	const LATER = 'crossroads.later';
	let later = $state(readLater());
	function readLater() {
		try {
			return JSON.parse(sessionStorage.getItem(LATER)) ?? [];
		} catch {
			return [];
		}
	}

	let dialog;
	let gift = $state(null); // what's on screen: { key, box } or { key, given }, a stamp
	let stamp = $state(null); // what came out of it
	let opened = $state([]); // keys opened here, before boxes and stamps reload
	let opening = $state(false);
	let error = $state('');

	const waiting = $derived(
		[
			...boxes.filter((b) => b.project_id && !b.opened_at).map((b) => ({ key: `box-${b.id}`, box: b })),
			...stamps.filter((s) => !s.seen_at).map((s) => ({ key: `stamp-${s.id}`, given: s }))
		].filter((g) => !opened.includes(g.key) && !later.includes(g.key))
	);
	const projectName = (id) => projects.find((p) => p.id === id)?.name ?? 'Your project';
	const name = $derived(gift?.box && projectName(gift.box.project_id));

	$effect(() => {
		if (gift || !waiting.length) return;
		gift = waiting[0];
		stamp = null;
		if (!dialog.open) dialog.showModal();
	});

	async function open() {
		error = '';
		opening = true;
		const { key, box, given } = gift;
		// Let the shut box rock for a moment, however fast the server is.
		const wait = new Promise((r) => setTimeout(r, 900));
		try {
			// A given stamp is already theirs: opening it only marks it seen.
			const got = await api('POST', box ? `/boxes/${box.id}/open` : `/special-stamps/${given.id}/seen`);
			await wait;
			opened.push(key);
			stamp = got;
			rail.stampsVersion++; // the stamp car in the train shows the new stamp
			onopened?.();
		} catch (e) {
			error = e.message;
		} finally {
			opening = false;
		}
	}

	// Closing (Later, Done or Escape) puts off whatever's still waiting.
	function closed() {
		later = [...later, ...waiting.map((g) => g.key)];
		try {
			sessionStorage.setItem(LATER, JSON.stringify(later));
		} catch {}
		gift = stamp = null;
	}
</script>

<dialog class="takeover" bind:this={dialog} aria-labelledby="takeover-title" oncancel={(e) => opening && e.preventDefault()} onclose={closed}>
	{#if gift}
		<div class="stack scene">
			{#if gift.box}
				<p class="kicker">Shipped and approved</p>
				<h1 id="takeover-title">{name} came with a stamp box!</h1>
			{:else}
				<p class="kicker">Special delivery</p>
				<h1 id="takeover-title">The organizers sent you a stamp!</h1>
			{/if}
			<div class="spot"><span class="light" class:out={opening || stamp}></span><BoxOpening {stamp} label={gift.box ? `${name}'s stamp box` : 'A box from the organizers'} /></div>
			{#if error}<p class="callout warn small">{error}</p>{/if}
			{#if stamp}
				<div class="got" role="status">
					<p class="won">You got <strong>{stamp.type.name}</strong></p>
					<p>{stamp.type.rarity} · {stamp.type.description || 'A collectible stamp.'}</p>
					<p class="small muted">It's in your stamp box: drag it onto one of your projects from the project's page.</p>
				</div>
				<!-- Out with the stamp's name, once it's out of the box. -->
				<div class="row actions got">
					{#if waiting.length}
						<button class="btn" onclick={() => (gift = null)}>
							{waiting[0].box ? `Open ${projectName(waiting[0].box.project_id)}'s box` : 'Open the next stamp'}
						</button>
					{/if}
					<button class="btn primary" onclick={() => dialog.close()}>Done</button>
				</div>
			{:else}
				<p class="muted">{gift.box ? "It got enough hours approved to earn its own box. What's inside?" : 'It came in a box. Which one could it be?'}</p>
				<div class="row actions">
					<button class="btn primary big" disabled={opening} onclick={open}>{opening ? 'Opening…' : 'Open it'}</button>
					<button class="btn" disabled={opening} onclick={() => dialog.close()}>Later</button>
				</div>
			{/if}
		</div>
	{/if}
</dialog>

<style>
	.takeover {
		width: 100vw;
		height: 100dvh;
		max-width: none;
		max-height: none;
		margin: 0;
		padding: 32px 16px;
		color: var(--text);
		background: var(--car-bg);
		border: 0;
		font-family: 'Crimson Pro', Georgia, serif;
		text-align: center;
		overflow-y: auto;
	}
	.takeover[open] { display: grid; place-items: center; }
	.takeover::backdrop { background: #000; }
	/* Home doesn't scroll behind it. */
	:global(html:has(.takeover[open])) { overflow: hidden; }
	.scene {
		align-items: center;
		max-width: 720px;
		animation: arrive 600ms ease-out both;
	}
	/* A spotlight on the box: the rest of the screen falls off into the dark,
	   until they open it and the dark recedes. It's centered on the box (near
	   the bottom of the 380px stage) and as wide as the screen on a phone. */
	@property --reach {
		syntax: '<percentage>';
		inherits: true;
		initial-value: 50%;
	}
	.spot { position: relative; width: 100%; }
	.light {
		position: absolute;
		inset: 0;
		z-index: -1;
		pointer-events: none;
		transition: opacity 1.2s ease-out, --reach 1.2s ease-out;
	}
	.light.out { --reach: 100%; opacity: 0; }
	.light::before,
	.light::after {
		content: '';
		position: absolute;
		left: 50%;
		top: -20px;
		width: min(100% + 32px, 760px);
		height: 560px;
		translate: -50% 0;
	}
	.light::before { background: radial-gradient(closest-side, rgb(255 236 200 / 0.1), transparent var(--reach), #000); }
	/* Its shadow darkens everything past the light without making the
	   takeover scroll. It starts a little inside the light, so there's no
	   seam between them. */
	.light::after { scale: 0.99; box-shadow: 0 0 0 200vmax #000; }
	.kicker {
		margin: 0;
		font-size: 18px;
		letter-spacing: 0.18em;
		text-transform: uppercase;
		color: var(--muted);
	}
	h1 { font-size: clamp(40px, 6vw, 72px); line-height: 1.05; text-wrap: balance; }
	.scene > p { margin: 0; font-size: 24px; }
	.got p { margin: 0 0 4px; font-size: 20px; }
	.got .won { font-size: 34px; }
	.got { animation: got-in 500ms ease-out 1900ms both; }
	.actions { justify-content: center; min-height: 44px; }
	.big { font-size: 34px; padding-inline: 36px; }
	@keyframes arrive { from { opacity: 0; translate: 0 24px; } }
	@keyframes got-in { from { opacity: 0; translate: 0 6px; } }
	@media (prefers-reduced-motion: reduce) {
		.scene, .got { animation: none; }
	}
</style>
