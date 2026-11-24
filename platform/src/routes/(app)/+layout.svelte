<script>
	// Every page in (app) needs someone signed in.
	import { onMount } from 'svelte';
	import { goto, beforeNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { session, refresh } from '#lib/session.svelte.js';
	import TopBar from '#lib/TopBar.svelte';
	import DevPanel from '#lib/DevPanel.svelte';
	import LandingPagePrompt from '#lib/LandingPagePrompt.svelte';
	import RailTrain from '#lib/RailTrain.svelte';
	import CommunityBridge from '#lib/CommunityBridge.svelte';
	import { rail, STATIONS } from '#lib/rail.svelte.js';
	import { getAhead } from '#lib/ahead.js';

	let { children } = $props();
	let error = $state('');
	// Home, Shop and Boxes share one long train (see RailTrain.svelte).
	const station = $derived(STATIONS.includes(page.url.pathname) ? page.url.pathname : null);
	// Reviewing a ship takes the whole window: the page has its own slim bar
	// instead of the site's.
	const reviewing = $derived(page.route.id === '/(app)/review/[id]');

	// From the first move between pages on, the train rolls in when it appears.
	beforeNavigate(() => {
		rail.traveled = true;
	});

	// Fetch a page's data as its link is pointed at, and follow the top bar's
	// and project rows' links on the press (see ahead.js).
	onMount(getAhead);

	onMount(async () => {
		try {
			if (!(await refresh())) goto('/signin?next=' + encodeURIComponent(page.url.pathname), { replace: true });
		} catch (e) {
			error = e.message;
		}
	});
</script>

<div class="app-shell">
	{#if error}
		<main class="main"><p class="callout warn">Couldn't reach the backend: {error} Is it running on :8080?</p></main>
	{:else if session.me && reviewing}
		{@render children()}
		<DevPanel onchange={refresh} />
	{:else if !reviewing}
		<!-- The top bar doesn't wait to know who's signed in: it's there as
		     soon as the page is, and fills in its right-hand side after. -->
		<TopBar />
		{#if session.me}
			<!-- The train is the first thing on Home, Shop and Boxes, at the same
			     place and size on each, so moving between them only slides it. -->
			{#if station}<RailTrain at={station} />{/if}
			<main class="main">
				{@render children()}
			</main>
			{#if station === '/home'}<CommunityBridge />{/if}
			<LandingPagePrompt />
			<DevPanel onchange={refresh} />
		{/if}
	{/if}
</div>
