<script>
	// A claim link, like the one in a Slack announcement: claims the stamp as
	// soon as the page opens. The (app) layout signs people in first and
	// brings them back here.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api } from '#lib/api.js';

	let claimed = $state(null);
	let error = $state('');

	onMount(async () => {
		try {
			claimed = await api('POST', '/special-stamps/claims', { code: page.params.code });
		} catch (e) {
			error = e.message;
		}
	});

	const stamp = $derived(claimed?.special_stamp);
</script>

<div class="stack-lg narrow">
	{#if error}
		<p class="callout warn">{error}</p>
	{:else if stamp}
		<section class="callout good drop" role="status">
			{#if stamp.type.image_url}<img class="drop-stamp" src={stamp.type.image_url} alt="" />{/if}
			<div class="stack-sm">
				<strong>{claimed.already_claimed ? `You already have ${stamp.type.name}.` : `You got: ${stamp.type.name}`}</strong>
				<span class="small">{stamp.type.rarity} · {stamp.type.description || 'A collectible stamp.'} It's in your stamp box: drag it onto one of your projects from the project's page.</span>
			</div>
		</section>
		<div class="row"><a class="btn primary" href="/home">Go home</a></div>
	{:else}
		<p class="muted">Claiming your stamp…</p>
	{/if}
</div>

<style>
	.drop { flex-direction: row; align-items: center; gap: 18px; }
	.drop-stamp { height: 120px; width: auto; flex: none; rotate: -3deg; filter: drop-shadow(0 2px 6px rgb(0 0 0 / 0.5)); }
</style>
