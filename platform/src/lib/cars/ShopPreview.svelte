<script>
	// The ticket car while you're on Home or Boxes: the same car Shop shows
	// (ShopCar), drawn from the shop items last loaded, so it doesn't change
	// as the train pulls away from Shop or back to it. Its buttons do nothing
	// here; the train keeps cars that aren't the page's own inert.
	import { get, peek } from '../api.js';
	import ShopCar from './ShopCar.svelte';

	let items = $state(peek('/shop/items') ?? null);
	let innerWidth = $state(1440);
	$effect(() => {
		get('/shop/items')
			.then((i) => (items = i))
			.catch(() => {});
	});
	const ticket = $derived(items?.find((i) => i.kind === 'ticket') ?? null);
	const lock = $derived(items?.find((i) => i.kind === 'price_lock') ?? null);
</script>

<svelte:window bind:innerWidth />

{#if ticket}
	<ShopCar {ticket} {lock} wide={innerWidth > 960} />
{:else}
	<!-- Before the shop has loaded once. -->
	<div class="blank stack-sm">
		<span class="kind">The trip</span>
		<h2>Crossroads rail pass</h2>
		<span>Berlin, then across Europe by train · February 2027</span>
	</div>
{/if}

<style>
	.blank { justify-content: center; height: 100%; padding: 20px 28px 22px; }
	.kind { font-size: 24px; }
	h2 { font-size: clamp(40px, 4vw, 64px); }
</style>
