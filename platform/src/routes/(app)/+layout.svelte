<script>
	// Every page in (app) needs someone signed in.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { session, refresh } from '#lib/session.svelte.js';
	import TopBar from '#lib/TopBar.svelte';
	import DevPanel from '#lib/DevPanel.svelte';

	let { children } = $props();
	let error = $state('');

	onMount(async () => {
		try {
			if (!(await refresh())) goto('/?next=' + encodeURIComponent(page.url.pathname), { replaceState: true });
		} catch (e) {
			error = e.message;
		}
	});
</script>

{#if error}
	<main class="main"><p class="callout warn">Couldn't reach the backend: {error} Is it running on :8080?</p></main>
{:else if session.me}
	<TopBar />
	<main class="main">
		{@render children()}
	</main>
	<DevPanel onchange={refresh} />
{/if}
