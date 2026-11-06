<script>
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { session, refresh } from '#lib/session.svelte.js';
	import { stampsPerHour } from '#lib/format.js';
	import Train from '#lib/Train.svelte';

	let items = $state([]);
	let error = $state('');
	let message = $state(null); // { tone, text, verify? }
	let buying = $state(null);
	// The buy popup: which item, and (for physical items) a different address if they want one.
	let dialog;
	let pending = $state(null); // the item being bought
	let other = $state(null); // null = ship to the Hack Club address
	let dialogError = $state('');
	const blank = () => ({ name: '', line_1: '', line_2: '', city: '', state: '', postal_code: '', country: '' });
	const ADDRESS_FIELDS = [['name', 'Name'], ['line_1', 'Address line 1'], ['line_2', 'Address line 2 (optional)'], ['city', 'City'], ['state', 'State / region'], ['postal_code', 'Postal code'], ['country', 'Country']];

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
	// Everything else, in the order the API gives: by each item's position (most wanted first).
	const others = $derived(items.filter((i) => i.kind !== 'ticket'));
	const rate = $derived(stampsPerHour(ticket));
	const stamps = $derived(session.me.stamps);
	const hasTicket = $derived(!!ticket && ticket.bought > 0);
	const address = $derived(session.me.address);
	const hoursFor = (n) => Math.ceil(n / rate);

	function soldOutReason(item) {
		if (item.max_per_user != null && item.bought >= item.max_per_user) return item.kind === 'ticket' ? 'You have your ticket' : 'You have the most you can get';
		if (item.stock === 0) return 'Sold out';
		return '';
	}

	// What buying would leave them short for the ticket, if it would.
	function ticketAfter(item) {
		if (item.kind === 'ticket' || !ticket || hasTicket) return null;
		const after = ticket.price - (stamps - item.price);
		return after > 0 ? after : null;
	}

	function ask(item) {
		pending = item;
		// No address on Hack Club Auth: start with the form open.
		other = item.kind === 'physical' && !address ? blank() : null;
		dialogError = '';
		message = null;
		dialog.showModal();
	}

	async function buy(e) {
		e.preventDefault();
		const item = pending;
		dialogError = '';
		buying = item.id;
		try {
			const body = item.kind === 'physical' && other ? { shipping_address: other } : undefined;
			const order = await api('POST', `/shop/items/${item.id}/orders`, body);
			message = {
				tone: 'good',
				text: item.kind === 'ticket' ? `You have your ticket (order #${order.id}). You've qualified for the trip!` : `Ordered ${item.name} (order #${order.id}). Track it in My orders.`
			};
			dialog.close();
			await Promise.all([load(), refresh()]);
		} catch (err) {
			// Address problems are fixable in the popup; anything else goes on the page.
			if (err.status === 422) dialogError = err.message;
			else {
				dialog.close();
				message = { tone: 'warn', text: err.message, verify: err.status === 403 };
				if (err.status === 409) await load();
			}
		} finally {
			buying = null;
		}
	}

	const lines = (a) => [a.name, a.line_1, a.line_2, [a.city, a.state, a.postal_code].filter(Boolean).join(', '), a.country].filter(Boolean);
</script>

<div class="shop">
	<div class="page-head">
		<div class="stack-sm">
			<h1>Shop</h1>
			<p>You have {stamps} stamps. You earn {rate} per approved hour.</p>
		</div>
		<a class="btn ghost" href="/shop/orders">My orders</a>
	</div>

	{#if error}<p class="callout warn">{error}</p>{/if}
	<!-- Always in the page so screen readers hear new messages; takes no space while empty. -->
	<div class="status" role="status" aria-live="polite">
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
		<Train labelledby="ticket-name">
		<div class="ticket">
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
							<button class="btn primary" disabled={buying === ticket.id} onclick={() => ask(ticket)}>{buying === ticket.id ? 'Buying…' : `Buy your ticket for ${ticket.price}`}</button>
						{/if}
						<span class="small">Buying the ticket is how you qualify. Everything else here spends stamps the ticket needs.</span>
					{/if}
				</div>
		</div>
		</Train>
	{/if}

	<section class="stack">
		{#if ticket && others.length}<h2>Everything else</h2>{/if}
		<div class="items">
			{#each others as item}
				{@const reason = soldOutReason(item)}
				{@const short = item.price - stamps}
				<article class="card item">
					<div class="thumb">
						{#if item.image_url}<img src={item.image_url} alt="" loading="lazy" />{/if}
					</div>
					<div class="info">
						<h3 class="item-name">{item.name}</h3>
						<span class="price">{item.price} stamps</span>
						{#if item.description}<p class="desc">{item.description}</p>{/if}
					</div>
					<!-- Pinned to the bottom so the buttons line up across a row. -->
					<div class="card-foot">
						{#if reason}
							<button class="btn" disabled>{reason}</button>
						{:else if short > 0}
							<button class="btn" disabled>Need {short} more stamps (~{hoursFor(short)}h)</button>
						{:else}
							<!-- Secondary until they have the ticket, so the trip stays the obvious choice. -->
							<button class="btn {hasTicket || !ticket ? 'primary' : 'ghost'}" disabled={buying === item.id} onclick={() => ask(item)}>{buying === item.id ? 'Buying…' : `Buy for ${item.price}`}</button>
						{/if}
					</div>
				</article>
			{:else}
				{#if !ticket}<p class="muted">Nothing in the shop yet.</p>{/if}
			{/each}
		</div>
	</section>
</div>

<dialog bind:this={dialog} aria-labelledby="buy-title" onclose={() => (pending = null)}>
	{#if pending}
		{@const left = stamps - pending.price}
		{@const short = ticketAfter(pending)}
		<form class="stack" method="dialog" onsubmit={buy}>
			<h2 id="buy-title">Buy {pending.name}?</h2>
			<dl class="sums">
				<dt>Price</dt><dd>{pending.price} stamps</dd>
				<dt>You have</dt><dd>{stamps} stamps</dd>
				<dt>Left after</dt><dd><b>{left} stamps</b></dd>
			</dl>
			{#if pending.kind === 'ticket'}
				<p>Buying the ticket is how you qualify for the trip.</p>
			{:else if short}
				<p class="callout warn">Your ticket costs {ticket.price}. After this you'd need {short} more stamps (about {hoursFor(short)} approved hours) to buy it.</p>
			{/if}

			{#if pending.kind === 'physical'}
				<div class="stack-sm">
					<span class="ship-label">Ship to</span>
					{#if other}
						<div class="addr-form">
							{#each ADDRESS_FIELDS as [k, label]}
								<input placeholder={label} aria-label={label} required={k !== 'line_2' && k !== 'state' && k !== 'postal_code'} bind:value={other[k]} />
							{/each}
						</div>
						{#if address}<button type="button" class="link-btn" onclick={() => (other = null)}>Use my Hack Club address instead</button>{/if}
					{:else}
						<address class="addr">{#each lines(address) as l}<span>{l}</span>{/each}</address>
						<span class="small">This is the address on your Hack Club account. Want it sent somewhere else? <button type="button" class="link-btn" onclick={() => (other = blank())}>Use a different address</button></span>
					{/if}
				</div>
			{/if}

			{#if dialogError}<p class="callout warn" role="alert">{dialogError}</p>{/if}
			<div class="row" style="justify-content: flex-end">
				<button type="button" class="btn ghost" onclick={() => dialog.close()}>Cancel</button>
				<button class="btn primary" disabled={buying === pending.id}>{buying === pending.id ? 'Buying…' : `Buy for ${pending.price}`}</button>
			</div>
		</form>
	{/if}
</dialog>

<style>
	.shop { display: flex; flex-direction: column; gap: 24px; }
	.status:not(:has(p)) { margin-top: -24px; }
	/* ---- Items: the site's textured cards, kept short ---- */
	.items {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
		gap: 16px;
	}
	.item {
		display: grid;
		grid-template-columns: 104px minmax(0, 1fr);
		grid-template-rows: 1fr auto;
		gap: 12px 16px;
		padding: 16px;
	}
	.thumb {
		width: 104px;
		height: 104px;
		overflow: hidden;
		background: var(--navy-deep);
		border-radius: 14px;
	}
	.thumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
	.info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.item .item-name { font-size: 28px; line-height: 1.1; }
	.item .price { font-size: 22px; }
	.desc {
		margin: 0;
		font-size: 20px;
		color: var(--muted);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.item .card-foot { grid-column: 1 / -1; padding-top: 0; }

	dialog {
		width: min(560px, calc(100vw - 32px));
		padding: 24px 28px;
		color: var(--text);
		background: var(--row-bg);
		border: 0;
		border-radius: var(--radius);
		box-shadow: 0 24px 60px rgb(0 0 0 / 0.6);
	}
	dialog::backdrop { background: rgb(0 0 0 / 0.7); }
	/* Ghost buttons on the dark popup get the lighter grey outline. */
	dialog .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); }
	dialog h2 { font-size: 40px; }
	.sums { display: grid; grid-template-columns: auto 1fr; gap: 6px 20px; margin: 0; font-size: 26px; }
	.sums dt { color: var(--muted); }
	.sums dd { margin: 0; text-align: right; }
	.sums b { font-weight: 400; padding: 2px 12px; background: var(--navy); border-radius: 8px; }
	.ship-label { font-size: 26px; }
	.addr { display: flex; flex-direction: column; padding: 12px 16px; font-style: normal; font-size: 23px; background: var(--navy-deep); border-radius: 10px; }
	.addr-form { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
	.addr-form input { font-size: 22px; }
	.addr-form input:nth-child(-n + 3) { grid-column: 1 / -1; }

	/* The ticket rides the train, like the home hero; Train draws the car. */
	.ticket {
		display: grid;
		grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
		gap: 20px 40px;
		align-items: center;
		padding: 24px 32px 28px;
	}
	.ticket h2 { font-size: clamp(40px, 4vw, 64px); }
	.ticket .kind { font-size: 24px; }
	.ticket .bar { height: 20px; }
	.ticket .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); align-self: flex-start; }
	.big-line { font-size: 36px; }
	.card-foot { margin-top: auto; padding-top: 8px; display: flex; }
	.card-foot .btn { width: 100%; white-space: normal; }
	@media (max-width: 820px) {
		.ticket { grid-template-columns: 1fr; padding-inline: 18px; }
	}
</style>
