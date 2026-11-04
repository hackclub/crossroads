<script>
	// Only shown when the backend runs with DEV_MODE=1. Switch test users and
	// add fake Hackatime time. Remove with dev mode (planning/DEV_MODE.md).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { devHackatime, devLoginUrl, isDevMode } from './api.js';

	let { onchange } = $props();
	let dev = $state(false);
	let open = $state(false);
	let time = $state([]);
	let name = $state('crossroads-game');
	let addHours = $state(5);
	let error = $state('');

	const users = [
		['alice', 'participant'],
		['bob', 'participant'],
		['rita', 'reviewer'],
		['adam', 'admin']
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
	<div class="devpanel">
		<button class="link-btn" onclick={() => { open = !open; if (open) load(); }}>
			{open ? '▾' : '▸'} Dev mode
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
				{#if time.length}
					<span class="small muted">Your Hackatime: {time.map((t) => `${t.name} ${(t.seconds / 3600).toFixed(1)}h`).join(' · ')}</span>
				{/if}
				{#if error}<span class="small" style="color: var(--danger)">{error}</span>{/if}
			</div>
		{/if}
	</div>
{/if}
