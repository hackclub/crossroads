<script>
	// The ticket car of the train: the trip, how close they are to the rail
	// pass (or that they have it), and on a wide screen the batches and the
	// price lock (ShopLadder).
	//
	// Shop hands it to the train with buying wired up; on Home and Boxes the
	// train draws the same car from the shop items last loaded (ShopPreview).
	// It is one component so the car looks the same either way: it doesn't
	// change shape as the train starts to move.
	import { session } from '../session.svelte.js';
	import ShopLadder, { hasLadder } from './ShopLadder.svelte';

	// live: Shop is open, so buttons work. wide: room for the ladder's column.
	let { ticket, lock = null, buying = null, onask, live = false, wide = true } = $props();

	const stamps = $derived(session.me?.stamps ?? 0);
	const hoursFor = (n) => Math.ceil(n / (session.me?.stamps_per_hour || 1));
	const hasTicket = $derived(ticket.bought > 0);
	const short = $derived(ticket.price - stamps);
	const pct = $derived(Math.min(Math.max(stamps / ticket.price, 0), 1) * 100);
	const laddered = $derived(wide && hasLadder(ticket));
	const soldOut = $derived(ticket.max_per_user != null && ticket.bought >= ticket.max_per_user ? 'You have your rail pass' : ticket.stock === 0 ? 'Sold out' : '');
</script>

<!-- Up to three columns side by side, so the car stays as low as the other
     cars of the train: the trip, your progress, then the batches and the
     price lock. -->
<div class="ticket" class:laddered>
	<div class="trip stack-sm">
		<span class="kind">The trip</span>
		<h2 id={live ? 'ticket-name' : undefined}>{ticket.name}</h2>
		<span>Berlin, then across Europe by train · February 2027</span>
		{#if ticket.description}<span class="small">{ticket.description}</span>{/if}
		{#if !hasTicket}<span class="small">Buying the rail pass is how you qualify. Everything else here spends tickets the rail pass needs.</span>{/if}
	</div>
	<div class="progress stack">
		{#if hasTicket}
			<p class="big-line">You have your rail pass. See you in Berlin!</p>
			<a class="btn ghost" href="/shop/orders">See your order</a>
		{:else}
			<div class="row between"><span>{Math.max(stamps, 0)} / {ticket.price} tickets</span><span>{short > 0 ? `${short} to go · about ${hoursFor(short)}h` : 'You can buy it now'}</span></div>
			<div class="bar" role="img" aria-label="{Math.round(pct)}% of the way to your rail pass"><span class="seg approved" style="width: {pct}%"></span></div>
			{#if soldOut}
				<button class="btn" disabled>{soldOut}</button>
			{:else if short > 0}
				<button class="btn" disabled>Buy for {ticket.price}</button>
			{:else}
				<button class="btn primary" disabled={buying === ticket.id} onclick={() => onask?.(ticket)}>{buying === ticket.id ? 'Buying…' : `Buy your rail pass for ${ticket.price}`}</button>
			{/if}
		{/if}
	</div>
	{#if laddered}<ShopLadder {ticket} {lock} {buying} {onask} />{/if}
</div>

<style>
	/* The ticket car of the long train; RailTrain draws the car. */
	.ticket {
		display: grid;
		grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
		gap: 18px 32px;
		align-items: center;
		padding: 20px 28px 22px;
	}
	/* With batches and the lock: three columns. The lock's note sits under
	   its title and button, so the column stays short. */
	.ticket.laddered { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.15fr); }
	.ticket h2 { font-size: clamp(40px, 4vw, 64px); }
	.ticket.laddered h2 { font-size: clamp(36px, 3vw, 52px); }
	.ticket .kind { font-size: 24px; }
	.ticket .bar { height: 20px; }
	.ticket .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); align-self: flex-start; }
	.big-line { font-size: 36px; }
	@media (max-width: 820px) {
		.ticket { grid-template-columns: 1fr; padding-inline: 18px; }
	}
</style>
