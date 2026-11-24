<script module>
	// Whether there's a ladder to show: batches or a locked price, until they
	// have their rail pass.
	export const hasLadder = (ticket) => !!ticket && !(ticket.bought > 0) && !!(ticket.batches || ticket.locked_price);
</script>

<script>
	// The rail pass's batches (which price is on sale, what comes next) and
	// the price lock. On a wide screen it's the third column of the ticket car
	// (ShopCar); on a narrow one it sits on the Shop page under the train, so
	// the car stays as low as the other cars.
	import { session } from '../session.svelte.js';
	import { stampHours } from '../format.js';

	let { ticket, lock = null, buying = null, onask } = $props();

	const stamps = $derived(session.me?.stamps ?? 0);
	const inHours = (n) => stampHours(n, session.me?.stamps_per_hour);
	const batches = $derived(ticket.batches);
	const lastBatch = $derived(!!batches && batches.current === batches.prices.length - 1);
	const canLock = $derived(!!lock && !!batches && !ticket.locked_price && ticket.bought === 0 && ticket.stock > 0 && !lastBatch);
</script>

<div class="ladder stack-sm">
	{#if batches}
		<ol class="batches" aria-label="Rail pass batches, {batches.size} passes each">
			{#each batches.prices as price, i}
				<li class:sold={i < batches.current} class:now={i === batches.current} aria-current={i === batches.current ? 'step' : undefined}>
					{inHours(price)}<span class="visually-hidden">{i < batches.current ? ', sold out' : i === batches.current ? ', on sale now' : ''}</span>
				</li>
			{/each}
		</ol>
		<span class="small">
			{#if ticket.stock === 0}
				Every batch has sold out.
			{:else}
				Batch {batches.current + 1} of {batches.prices.length}: {batches.left_in_batch} left at {inHours(batches.prices[batches.current])}.
				{lastBatch ? 'This is the last batch.' : `Then ${inHours(batches.prices[batches.current + 1])}.`}
			{/if}
		</span>
	{/if}
	{#if ticket.locked_price}
		<p class="callout info small">Your price is locked at {inHours(ticket.locked_price)} ({ticket.locked_price} tickets).</p>
	{/if}
	{#if canLock}
		<div class="lock">
			<div class="lock-head">
				<span>Lock in {inHours(batches.prices[batches.current])} for {inHours(lock.price)}</span>
				{#if stamps < lock.price}
					<button class="btn" disabled>Need {lock.price - stamps} more</button>
				{:else}
					<button class="btn ghost" disabled={buying === lock.id} onclick={() => onask?.(lock)}>{buying === lock.id ? 'Locking…' : `Lock for ${lock.price}`}</button>
				{/if}
			</div>
			<span class="small">
				Keeps this price however many batches sell before you're ready. It's on top of the rail pass, and if the passes sell out
				before you buy one, you get the {lock.price} tickets back.
			</span>
		</div>
	{/if}
</div>

<style>
	.lock-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 12px; }
	.lock-head > span { font-size: 26px; }
	.btn.ghost { box-shadow: inset 0 0 0 2px var(--head); align-self: flex-start; }
	/* The batch ladder: sold ones crossed out, the one on sale filled. */
	.batches { display: flex; flex-wrap: wrap; gap: 6px; margin: 0; padding: 0; list-style: none; }
	.batches li { padding: 2px 12px; font-size: 22px; background: var(--navy-deep); border-radius: 8px; }
	.batches li.sold { color: var(--muted); text-decoration: line-through; }
	.batches li.now { background: var(--head); color: var(--ink); text-shadow: none; }
	.lock { display: flex; flex-direction: column; gap: 6px; padding: 12px 16px; background: rgb(0 0 0 / 0.35); border-radius: 14px; }
	.lock .btn { flex: none; }
	.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
</style>
