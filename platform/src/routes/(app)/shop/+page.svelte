<script>
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { session, refresh } from '#lib/session.svelte.js';
	import { money, stampsPerHour } from '#lib/format.js';

	let items = $state([]);
	let error = $state('');
	let message = $state(null); // { tone, text, verify? }
	let buying = $state(null);
	// Physical items: ship to the Hack Club address unless they type another.
	let custom = $state(null);
	const blank = () => ({ name: '', line_1: '', line_2: '', city: '', state: '', postal_code: '', country: '' });

	async function load() {
		try {
			items = await get('/shop/items');
		} catch (e) {
			error = e.message;
		}
	}
	onMount(load);

	// The ticket is the point of the shop, so it gets its own card first.
	const ticket = $derived(items.find((i) => i.kind === 'ticket'));
	const others = $derived(items.filter((i) => i.kind !== 'ticket').toSorted((a, b) => a.price - b.price));
	const rate = $derived(stampsPerHour(ticket));
	const stamps = $derived(session.me.stamps);
	const hasTicket = $derived(!!ticket && ticket.bought > 0);
	const kindLabel = { physical: 'Shipped to you', stipend: 'Travel stipend' };
	const hoursFor = (n) => Math.ceil(n / rate);

	function soldOutReason(item) {
		if (item.max_per_user != null && item.bought >= item.max_per_user) return item.kind === 'ticket' ? 'You have your ticket' : 'You have the most you can get';
		if (item.stock === 0) return 'Sold out';
		return '';
	}

	// Says what a purchase does to the trip, before they commit.
	function confirmText(item) {
		if (item.kind === 'ticket') return `Buy your Crossroads ticket for ${item.price} stamps? This is how you qualify for the trip.`;
		if (!ticket || hasTicket) return `Buy ${item.name} for ${item.price} stamps?`;
		const after = ticket.price - (stamps - item.price);
		return `Buy ${item.name} for ${item.price} stamps?\n\nYour ticket costs ${ticket.price}. After this you'd need ${after} more stamps (about ${hoursFor(after)} approved hours) to buy it.`;
	}

	async function buy(item) {
		if (!confirm(confirmText(item))) return;
		message = null;
		buying = item.id;
		try {
			const body = item.kind === 'physical' && custom?.itemId === item.id ? { shipping_address: custom.address } : undefined;
			const order = await api('POST', `/shop/items/${item.id}/orders`, body);
			message = {
				tone: 'good',
				text: item.kind === 'ticket' ? `You have your ticket (order #${order.id}). You've qualified for the trip!` : `Ordered ${item.name} (order #${order.id}). Track it in My orders.`
			};
			custom = null;
			await Promise.all([load(), refresh()]);
		} catch (e) {
			message = { tone: 'warn', text: e.message, verify: e.status === 403 };
			if (e.status === 409) await load();
		} finally {
			buying = null;
		}
	}
</script>

<div class="stack-lg">
	<div class="page-head">
		<div class="stack-sm">
			<h1>Shop</h1>
			<p>You have {stamps} stamps. You earn {rate} per approved hour.</p>
		</div>
		<a class="btn ghost" href="/shop/orders">My orders</a>
	</div>

	{#if error}<p class="callout warn">{error}</p>{/if}
	<div role="status" aria-live="polite">
		{#if message}
			<p class="callout {message.tone}">
				{message.text}
				{#if message.verify}<span>Verify your identity on <a href="https://auth.hackclub.com" target="_blank" rel="noreferrer">Hack Club Auth</a>, then try again.</span>{/if}
			</p>
		{/if}
	</div>

	{#if ticket}
		{@const short = ticket.price - stamps}
		{@const pct = Math.min(Math.max(stamps / ticket.price, 0), 1) * 100}
		<section class="ticket" aria-labelledby="ticket-name">
			<div class="stack-sm">
				<span class="kind">The trip</span>
				<h2 id="ticket-name">{ticket.name}</h2>
				<span>Paris, then across Europe by train · January 8–12</span>
				{#if ticket.description}<span class="small">{ticket.description}</span>{/if}
			</div>
			<div class="stack">
				{#if hasTicket}
					<p class="big-line">You have your ticket. See you in Paris!</p>
					<a class="btn ghost" href="/shop/orders">See your order</a>
				{:else}
					<div class="row between"><span>{Math.max(stamps, 0)} / {ticket.price} stamps</span><span>{short > 0 ? `${short} to go · about ${hoursFor(short)}h` : 'You can buy it now'}</span></div>
					<div class="bar" role="img" aria-label="{Math.round(pct)}% of the way to your ticket"><span class="seg approved" style="width: {pct}%"></span></div>
					{#if soldOutReason(ticket)}
						<button class="btn" disabled>{soldOutReason(ticket)}</button>
					{:else if short > 0}
						<button class="btn" disabled>Buy for {ticket.price}</button>
					{:else}
						<button class="btn primary" disabled={buying === ticket.id} onclick={() => buy(ticket)}>{buying === ticket.id ? 'Buying…' : `Buy your ticket for ${ticket.price}`}</button>
					{/if}
					<span class="small">Buying the ticket is how you qualify. Everything else here spends stamps the ticket needs.</span>
				{/if}
			</div>
		</section>
	{/if}

	<section class="stack">
		{#if ticket && others.length}<h2>Everything else</h2>{/if}
		<div class="grid">
			{#each others as item}
				{@const reason = soldOutReason(item)}
				{@const short = item.price - stamps}
				<div class="card">
					{#if item.image_url}<img src={item.image_url} alt="" style="aspect-ratio: 4/3; object-fit: cover; border-radius: 6px" />{/if}
					<span class="small">{kindLabel[item.kind]}{#if item.stipend_cents} · {money(item.stipend_cents)}{/if}</span>
					<span class="item-name">{item.name}</span>
					{#if item.description}<span class="small">{item.description}</span>{/if}
					<div class="row between">
						<span class="price">{item.price} stamps</span>
						<span class="small">{item.stock == null ? '' : `${item.stock} left`}</span>
					</div>
					{#if item.kind === 'physical' && !reason}
						{#if custom?.itemId === item.id}
							<div class="stack-sm small">
								{#each [['name', 'Name'], ['line_1', 'Address line 1'], ['line_2', 'Address line 2'], ['city', 'City'], ['state', 'State / region'], ['postal_code', 'Postal code'], ['country', 'Country']] as [k, label]}
									<input placeholder={label} aria-label={label} bind:value={custom.address[k]} style="background: var(--deep)" />
								{/each}
								<button class="link-btn" onclick={() => (custom = null)}>Use my Hack Club address instead</button>
							</div>
						{:else}
							<span class="small">Ships to the address on your Hack Club account. <button class="link-btn" onclick={() => (custom = { itemId: item.id, address: blank() })}>Use another address</button></span>
						{/if}
					{/if}
					<!-- Pinned to the bottom so the buttons line up across a row of cards. -->
					<div class="card-foot">
						{#if reason}
							<button class="btn" disabled>{reason}</button>
						{:else if short > 0}
							<button class="btn" disabled>Need {short} more stamps (~{hoursFor(short)}h)</button>
						{:else}
							<!-- Secondary until they have the ticket, so the trip stays the obvious choice. -->
							<button class="btn {hasTicket || !ticket ? 'primary' : 'ghost'}" disabled={buying === item.id} onclick={() => buy(item)}>{buying === item.id ? 'Buying…' : `Buy for ${item.price}`}</button>
						{/if}
					</div>
				</div>
			{:else}
				{#if !ticket}<p class="muted">Nothing in the shop yet.</p>{/if}
			{/each}
		</div>
	</section>
</div>

<style>
	.ticket {
		display: grid;
		grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
		gap: 20px 40px;
		align-items: center;
		padding: 24px 32px 28px;
		background: var(--head);
		color: var(--ink); /* black on #8c8c8c is 6.25:1; white is only 3.36 */
		border-radius: var(--radius);
	}
	.ticket .btn { color: var(--text); }
	.ticket a:not(.btn) { color: var(--ink); }
	.ticket h2 { font-size: clamp(40px, 4vw, 64px); }
	.ticket .kind { font-size: 24px; }
	.ticket .bar { height: 20px; }
	.ticket .btn.primary { background: var(--deepest); }
	.ticket .btn:disabled { background: var(--body); }
	.ticket .btn.ghost { box-shadow: inset 0 0 0 2px var(--deepest); align-self: flex-start; }
	.big-line { font-size: 36px; }
	.item-name { font-size: 30px; line-height: 1.1; }
	.price { font-size: 26px; }
	.card-foot { margin-top: auto; padding-top: 8px; display: flex; }
	.card-foot .btn { width: 100%; white-space: normal; }
	@media (max-width: 820px) {
		.ticket { grid-template-columns: 1fr; padding-inline: 18px; }
	}
</style>
