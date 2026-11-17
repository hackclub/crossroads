<script>
	// Only shown when the backend runs with DEV_MODE=1. Switch test users, add
	// fake Hackatime time, and give yourself boxes. Remove with dev mode
	// (planning/DEV_MODE.md).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { devGiveBox, devHackatime, devLoginUrl, isDevMode } from './api.js';

	let { onchange } = $props();
	let dev = $state(false);
	let open = $state(false);
	let time = $state([]);
	let name = $state('crossroads-game');
	let addHours = $state(5);
	let error = $state('');
	let boxNote = $state('');
	let giving = $state(false);

	const users = [
		['alice', 'participant'],
		['bob', 'participant'],
		['rita', 'reviewer'],
		['adam', 'admin'],
		['sam', 'superadmin']
	];

	onMount(async () => {
		dev = await isDevMode();
	});

	async function load() {
		error = '';
		try {
			time = await devHackatime();
		} catch (e) {
			error = e.message;
		}
	}

	// A box to open on the Boxes page, which drops a stamp into your stamp box.
	async function giveBox() {
		error = '';
		giving = true;
		try {
			await devGiveBox();
			boxNote = 'Added a box. Open it on the Boxes page.';
			onchange?.();
		} catch (err) {
			error = err.message;
		} finally {
			giving = false;
		}
	}

	async function add(e) {
		e.preventDefault();
		error = '';
		try {
			time = await devHackatime({ name, seconds: Math.round(addHours * 3600) });
			onchange?.();
		} catch (err) {
			error = err.message;
		}
	}
</script>

{#if dev}
	<!-- Closed, it's a small tab in the corner so it doesn't sit on the page. -->
	<div class="devpanel" class:closed={!open}>
		<button class="link-btn" onclick={() => { open = !open; if (open) load(); }} title="Test tools: sign in as someone, fake Hackatime, boxes" aria-expanded={open}>
			{open ? '▾ Dev mode' : 'Dev'}
		</button>
		{#if open}
			<div class="stack-sm">
				<div class="row small">
					<strong>Sign in as:</strong>
					{#each users as [n, role]}
						<a href={devLoginUrl(n, role, page.url.pathname)}>{n} ({role})</a>
					{/each}
				</div>
				<form class="row small" onsubmit={add}>
					<strong>Fake Hackatime:</strong>
					add <input type="number" min="0.1" step="0.1" bind:value={addHours} style="width: 70px" /> h to
					<input list="dev-projects" bind:value={name} style="width: 180px" />
					<datalist id="dev-projects">{#each time as t}<option value={t.name}></option>{/each}</datalist>
					<button class="btn small">Add</button>
				</form>
				<div class="row small">
					<strong>Stamp box:</strong>
					<button class="btn small" disabled={giving} onclick={giveBox}>{giving ? 'Adding…' : 'Add a box'}</button>
					{#if boxNote}<a href="/boxes">{boxNote}</a>{/if}
				</div>
				{#if time.length}
					<span class="small muted">Your Hackatime: {time.map((t) => `${t.name} ${(t.seconds / 3600).toFixed(1)}h`).join(' · ')}</span>
				{/if}
				{#if error}<span class="small" style="color: var(--danger)">{error}</span>{/if}
			</div>
		{/if}
	</div>
{/if}
