<script>
	import { page } from '$app/state';
	import Img from '#lib/components/Img.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, shopItems, PROGRAM } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	const item = $derived(shopItems.find((i) => i.id === page.params.id));
	let qty = $state(1);
	let ordered = $state(false);
	const total = $derived((item?.cost ?? 0) * qty);
	const blocked = $derived(item?.tripOnly && !d.qualified);
</script>

{#if !item}
	<h1>Item not found</h1>
{:else}
	<div class="crumbs"><a href="/shop">Shop</a> / {item.name}</div>
	<div class="grid-halves">
		<Img label={item.name} ratio="1 / 1" />
		<div class="stack">
			<span class="muted small">{item.category}</span>
			<h1>{item.name}</h1>
			<p class="big">{item.cost} <span class="small muted">{PROGRAM.currency}</span></p>
			<p>Item description goes here: what it is, specs, and anything to know before ordering.</p>
			{#if item.note}<p class="small muted">{item.note}</p>{/if}
			<p class="small muted">{item.stock == null ? 'Unlimited stock' : `${item.stock} left`} · Usually ships in 2–4 weeks · Customs fees may apply outside the US</p>

			{#if ordered}
				<div class="callout good">
					<strong>Order placed</strong>
					<p class="small">We'll check it and send it out. Track it in <a href="/shop/orders">My orders</a>.</p>
				</div>
			{:else}
				<label class="field" style="max-width: 120px"><span>Quantity</span><input type="number" min="1" max="5" bind:value={qty} /></label>
				<label class="field"><span>Note for us <em>optional</em></span><input type="text" placeholder="Size, color, anything we should know" /></label>
				<p class="small">Ships to the address on your Hack Club account. <a href="/settings">Check it</a></p>
				<div class="row">
					<button class="btn primary" disabled={total > d.balance || blocked} onclick={() => (ordered = true)}>Order for {total} {PROGRAM.currency}</button>
				</div>
				{#if blocked}
					<p class="small muted">Only for people who've qualified for the trip.</p>
				{:else if total > d.balance}
					<p class="small muted">You need {total - d.balance} more {PROGRAM.currency}. Keep building!</p>
				{/if}
			{/if}
		</div>
	</div>
{/if}
