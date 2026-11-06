<script>
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import People from '#lib/admin/People.svelte';
	import Orders from '#lib/admin/Orders.svelte';
	import ShopItems from '#lib/admin/ShopItems.svelte';
	import StampTypes from '#lib/admin/StampTypes.svelte';
	import BoxTypes from '#lib/admin/BoxTypes.svelte';
	import Milestones from '#lib/admin/Milestones.svelte';
	import Stats from '#lib/admin/Stats.svelte';

	const tabs = {
		people: ['People', People],
		stats: ['Stats', Stats],
		orders: ['Orders', Orders],
		shop: ['Shop items', ShopItems],
		stamps: ['Special stamp types', StampTypes],
		boxes: ['Box types', BoxTypes],
		milestones: ['Milestones', Milestones]
	};
	const tab = $derived(tabs[page.url.searchParams.get('tab')] ? page.url.searchParams.get('tab') : 'people');
	const Current = $derived(tabs[tab][1]);
</script>

<div class="page-head">
	<div class="stack-sm">
		<h1>Admin</h1>
		<p class="muted small">Setting up for launch: stamp types → box types and their drops → milestones → shop items (the ticket costs 1,800). See planning/INTEGRATING.md.</p>
	</div>
</div>

<div class="tabs">
	{#each Object.entries(tabs) as [key, [label]]}
		<button class:active={tab === key} onclick={() => goto(`?tab=${key}`, { replaceState: true, noScroll: true })}>{label}</button>
	{/each}
</div>

{#key tab}<Current />{/key}
