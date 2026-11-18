<script>
	// A project's stamp box (from a per-project milestone) takes over Home
	// until it's opened, so it's the first thing someone sees once the ship
	// that earned it is signed off. "Later" puts it off until they come back
	// in a new tab; it waits on the Boxes page meanwhile.
	import { api } from './api.js';
	import { rail } from './rail.svelte.js';
	import BoxOpening from './BoxOpening.svelte';

	// boxes: GET /boxes. projects: GET /projects, for the names. onopened
	// runs after each box opens, to reload them.
	let { boxes, projects, onopened } = $props();

	const LATER = 'crossroads.later-boxes';
	let later = $state(readLater());
	function readLater() {
		try {
			return JSON.parse(sessionStorage.getItem(LATER)) ?? [];
		} catch {
			return [];
		}
	}

	let dialog;
	let box = $state(null); // the box on screen
	let stamp = $state(null); // what came out of it
	let opened = $state([]); // ids opened here, before boxes reloads
	let opening = $state(false);
	let error = $state('');

	const waiting = $derived(boxes.filter((b) => b.project_id && !b.opened_at && !opened.includes(b.id) && !later.includes(b.id)));
	const projectName = (id) => projects.find((p) => p.id === id)?.name ?? 'Your project';
	const name = $derived(box && projectName(box.project_id));

	$effect(() => {
		if (box || !waiting.length) return;
		box = waiting[0];
		stamp = null;
		if (!dialog.open) dialog.showModal();
	});

	async function open() {
		error = '';
		opening = true;
		const id = box.id;
		// Let the shut box rock for a moment, however fast the server is.
		const wait = new Promise((r) => setTimeout(r, 900));
		try {
			const got = await api('POST', `/boxes/${id}/open`);
			await wait;
			opened.push(id);
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
		later = [...later, ...waiting.map((b) => b.id)];
		try {
			sessionStorage.setItem(LATER, JSON.stringify(later));
		} catch {}
		box = stamp = null;
	}
</script>

<dialog class="takeover" bind:this={dialog} aria-labelledby="takeover-title" oncancel={(e) => opening && e.preventDefault()} onclose={closed}>
	{#if box}
		<div class="stack scene">
			<p class="kicker">Shipped and approved</p>
			<h1 id="takeover-title">{name} came with a stamp box!</h1>
			<BoxOpening {stamp} label="{name}'s stamp box" />
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
						<button class="btn" onclick={() => (box = null)}>Open {projectName(waiting[0].project_id)}'s box</button>
					{/if}
					<button class="btn primary" onclick={() => dialog.close()}>Done</button>
				</div>
			{:else}
				<p class="muted">It got enough hours approved to earn its own box. What's inside?</p>
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
		background: radial-gradient(ellipse 60% 50% at 50% 58%, rgb(255 190 110 / 0.14), transparent 70%), var(--car-bg);
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
