<script>
	// The ticket car while you're on another page: how close you are to it.
	// The full ticket, with buying, comes from the Shop page (see Station.svelte).
	import { get } from '../api.js';
	import { session } from '../session.svelte.js';

	let ticket = $state(null);
	$effect(() => {
		get('/shop/items')
			.then((items) => (ticket = items.find((i) => i.kind === 'ticket') ?? null))
			.catch(() => {});
	});
	const stamps = $derived(session.me?.stamps ?? 0);
	const pct = $derived(ticket ? Math.min(Math.max(stamps / ticket.price, 0), 1) * 100 : 0);
</script>

<div class="preview">
	<span class="kind">The trip</span>
	<h2>{ticket?.name ?? 'Crossroads ticket'}</h2>
	<span>Berlin, then across Europe by train · February 2027</span>
	{#if ticket}
		{#if ticket.bought > 0}
			<span>You have your ticket. See you in Berlin!</span>
		{:else}
			<span class="small">{Math.max(stamps, 0)} / {ticket.price} stamps</span>
			<div class="bar" aria-hidden="true"><span class="seg approved" style="width: {pct}%"></span></div>
		{/if}
	{/if}
</div>

<style>
	.preview { display: flex; flex-direction: column; justify-content: center; gap: 6px; height: 100%; padding: 24px 32px 28px; }
	.kind { font-size: 24px; }
	h2 { font-size: clamp(40px, 4vw, 64px); }
	.bar { height: 20px; max-width: 520px; margin-top: 6px; }
</style>
