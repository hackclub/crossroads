<script>
	// Every page in (app) needs someone signed in.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { session, refresh } from '#lib/session.svelte.js';
	import TopBar from '#lib/TopBar.svelte';
	import DevPanel from '#lib/DevPanel.svelte';
	import RailTrain from '#lib/RailTrain.svelte';
	import CommunityBridge from '#lib/CommunityBridge.svelte';
	import { STATIONS } from '#lib/rail.svelte.js';

	let { children } = $props();
	let error = $state('');
	// Home, Shop and Boxes share one long train (see RailTrain.svelte).
	const station = $derived(STATIONS.includes(page.url.pathname) ? page.url.pathname : null);

	onMount(async () => {
		try {
			if (!(await refresh())) goto('/signin?next=' + encodeURIComponent(page.url.pathname), { replaceState: true });
		} catch (e) {
			error = e.message;
		}
	});
</script>

<div class="app-shell">
	{#if error}
		<main class="main"><p class="callout warn">Couldn't reach the backend: {error} Is it running on :8080?</p></main>
	{:else if session.me}
		<TopBar />
		<!-- Home opens with the community goal ("Repair Bridge"), above the train. -->
		{#if station}<CommunityBridge open={station === '/home'} />{/if}
		{#if station}<RailTrain at={station} />{/if}
		<main class="main">
			{@render children()}
		</main>
		<DevPanel onchange={refresh} />
	{/if}
</div>
