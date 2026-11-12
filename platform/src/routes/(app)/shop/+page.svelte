<script>
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { session, refresh } from '#lib/session.svelte.js';
	import { stampHours } from '#lib/format.js';
	import Station from '#lib/Station.svelte';
	import StampIcon from '#lib/StampIcon.svelte';

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

	// The ticket is the point of the shop, so it gets its own card first, with
	// the price lock for it.
	const ticket = $derived(items.find((i) => i.kind === 'ticket'));
	const lock = $derived(items.find((i) => i.kind === 'price_lock'));
	// Everything else, in the order the API gives: by each item's position (most wanted first).
	const others = $derived(items.filter((i) => i.kind !== 'ticket' && i.kind !== 'price_lock'));
	const rate = $derived(session.me.stamps_per_hour);
	const inHours = (n) => stampHours(n, rate);
	// Tickets sold in batches: which one is on sale, and whether a lock would help.
	const batches = $derived(ticket?.batches);
	const lastBatch = $derived(!!batches && batches.current === batches.prices.length - 1);
	const canLock = $derived(!!lock && !!batches && !ticket.locked_price && ticket.bought === 0 && ticket.stock > 0 && !lastBatch);
	const stamps = $derived(session.me.stamps);
	const hasTicket = $derived(!!ticket && ticket.bought > 0);
	const address = $derived(session.me.address);
	const hoursFor = (n) => Math.ceil(n / rate);

	// The launch sale: everything but the ticket and its lock is some percent
	// off until November 18. Prices go back up when it ends, so the page
	// reloads them then rather than showing prices it wouldn't charge.
	const sale = $derived(others.find((i) => i.sale)?.sale);
	$effect(() => {
		if (!sale) return;
		const timer = setTimeout(load, Math.min(Math.max(new Date(sale.ends_at) - Date.now(), 0) + 5000, 2 ** 31 - 1));
		return () => clearTimeout(timer);
	});
	const until = (iso) => new Date(iso).toLocaleString(undefined, { weekday: 'long', month: 'long', day: 'numeric', hour: 'numeric', minute: '2-digit' });
	// The sale sticker's starburst, as a clip-path: 18 points.
	const burst = `polygon(${Array.from({ length: 36 }, (_, i) => {
		const r = i % 2 ? 43 : 50;
		const a = (Math.PI * i) / 18;
		return `${(50 + r * Math.sin(a)).toFixed(2)}% ${(50 - r * Math.cos(a)).toFixed(2)}%`;
	}).join(', ')})`;

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
				text:
					item.kind === 'ticket'
						? `You have your ticket (order #${order.id}). You've qualified for the trip!`
						: item.kind === 'price_lock'
							? `Your ticket is locked at ${inHours(order.locked_price)} (${order.locked_price} stamps), whenever you buy it.`
							: `Ordered ${item.name} (order #${order.id}). Track it in My orders.`
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
			<p>You have {stamps}<StampIcon /> stamps. You earn {rate} per approved hour.</p>
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
		<!-- The ticket is a car of the train in the layout. -->
		<Station name="shop">
		<!-- Up to three columns side by side, so the car stays as low as the
		     other cars of the train: the trip, your progress, then the batches
		     and the price lock. -->
		<div class="ticket" class:laddered={!hasTicket && (batches || ticket.locked_price)}>
				<div class="trip stack-sm">
					<span class="kind">The trip</span>
					<h2 id="ticket-name">{ticket.name}</h2>
					<span>Berlin, then across Europe by train · February 2027</span>
					{#if ticket.description}<span class="small">{ticket.description}</span>{/if}
					{#if !hasTicket}<span class="small">Buying the ticket is how you qualify. Everything else here spends stamps the ticket needs.</span>{/if}
				</div>
				<div class="progress stack">
					{#if hasTicket}
						<p class="big-line">You have your ticket. See you in Berlin!</p>
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
					{/if}
				</div>
				{#if !hasTicket && (batches || ticket.locked_price)}
					<div class="ladder stack-sm">
						{#if batches}
							<ol class="batches" aria-label="Ticket batches, {batches.size} tickets each">
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
							<p class="callout info small">Your price is locked at {inHours(ticket.locked_price)} ({ticket.locked_price} stamps).</p>
						{/if}
						{#if canLock}
							<div class="lock">
								<div class="lock-head">
									<span>Lock in {inHours(batches.prices[batches.current])} for {inHours(lock.price)}</span>
									{#if stamps < lock.price}
										<button class="btn" disabled>Need {lock.price - stamps} more</button>
									{:else}
										<button class="btn ghost" disabled={buying === lock.id} onclick={() => ask(lock)}>{buying === lock.id ? 'Locking…' : `Lock for ${lock.price}`}</button>
									{/if}
								</div>
								<span class="small">
									Keeps this price however many batches sell before you're ready. It's on top of the ticket, and if the tickets sell out
									before you buy one, you get the {lock.price} stamps back.
								</span>
							</div>
						{/if}
					</div>
				{/if}
		</div>
		</Station>
	{/if}

	<section class="stack">
		{#if ticket && others.length}<h2>Everything else</h2>{/if}
		{#if sale}<p class="sale-note">Everything here is {sale.percent}% off until {until(sale.ends_at)}.</p>{/if}
		<div class="items" class:on-sale={sale}>
			{#each others as item}
				{@const reason = soldOutReason(item)}
				{@const short = item.price - stamps}
				<article class="card item">
					{#if item.sale}
						<span class="sticker" style:--burst={burst} aria-hidden="true"><span><b>{item.sale.percent}%</b>off</span></span>
					{/if}
					<div class="thumb">
						{#if item.image_url}<img src={item.image_url} alt="" loading="lazy" />{/if}
					</div>
					<div class="info">
						<h3 class="item-name">{item.name}</h3>
						{#if item.sale}
							<span class="price"><s aria-hidden="true">{item.sale.full_price}</s> {item.price} stamps<span class="visually-hidden"> ({item.sale.percent}% off, was {item.sale.full_price})</span></span>
						{:else}
							<span class="price">{item.price} stamps</span>
						{/if}
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
			<h2 id="buy-title">{pending.kind === 'price_lock' ? `Lock your ticket at ${inHours(batches.prices[batches.current])}?` : `Buy ${pending.name}?`}</h2>
			<dl class="sums">
				<dt>Price</dt><dd>{#if pending.sale}<s aria-hidden="true">{pending.sale.full_price}</s> {/if}{pending.price} stamps{#if pending.sale}<span class="visually-hidden"> ({pending.sale.percent}% off)</span>{/if}</dd>
				<dt>You have</dt><dd>{stamps} stamps</dd>
				<dt>Left after</dt><dd><b>{left} stamps</b></dd>
			</dl>
			{#if pending.kind === 'ticket'}
				<p>Buying the ticket is how you qualify for the trip.</p>
			{:else if pending.kind === 'price_lock'}
				<p>
					Your ticket will cost {batches.prices[batches.current]} stamps whenever you buy it, even after later batches cost more. The
					lock doesn't hold a ticket for you: if they sell out before you buy one, you get these {pending.price} stamps back.
				</p>
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
				<button class="btn primary" disabled={buying === pending.id}>{buying === pending.id ? 'Buying…' : `${pending.kind === 'price_lock' ? 'Lock' : 'Buy'} for ${pending.price}`}</button>
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
		position: relative;
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

	/* The launch sale: a die-cut sticker slapped over the top right corner of
	   everything on sale. A white vinyl edge round a glossy red starburst, in
	   heavy type. The name and price keep clear of it. */
	.sale-note { font-size: 24px; }
	.items.on-sale { padding-top: 16px; }
	.on-sale .item-name, .on-sale .price { padding-right: 80px; }
	.sticker {
		position: absolute;
		z-index: 1;
		top: -26px;
		right: -24px;
		display: grid;
		width: 112px;
		aspect-ratio: 1;
		rotate: 12deg;
		filter: drop-shadow(0 1px 1px rgb(0 0 0 / 0.5)) drop-shadow(0 6px 8px rgb(0 0 0 / 0.35));
		pointer-events: none;
	}
	.sticker::before, .sticker::after, .sticker > span { grid-area: 1 / 1; clip-path: var(--burst); }
	.sticker::before { content: ''; background: #fbf7ee; }
	.sticker > span {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		margin: 5px;
		color: #fff;
		background: radial-gradient(circle at 32% 28%, #ff6a45, #e0261a 55%, #a8130b);
		font: 900 17px/0.95 'Arial Black', 'Helvetica Neue', Impact, system-ui, sans-serif;
		font-size-adjust: none;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		text-shadow: 0 1px 0 rgb(0 0 0 / 0.3);
	}
	.sticker b { font-size: 34px; font-weight: 900; letter-spacing: -0.02em; }
	/* Gloss on the vinyl. */
	.sticker::after { content: ''; margin: 5px; background: linear-gradient(140deg, rgb(255 255 255 / 0.4), transparent 42%); }
	.price s, .sums s { color: var(--muted); }

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
	.lock-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 12px; }
	.lock-head > span { font-size: 26px; }
	.ticket .kind { font-size: 24px; }
	.ticket .bar { height: 20px; }
	.ticket .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); align-self: flex-start; }
	.big-line { font-size: 36px; }
	/* The batch ladder: sold ones crossed out, the one on sale filled. */
	.batches { display: flex; flex-wrap: wrap; gap: 6px; margin: 0; padding: 0; list-style: none; }
	.batches li { padding: 2px 12px; font-size: 22px; background: var(--navy-deep); border-radius: 8px; }
	.batches li.sold { color: var(--muted); text-decoration: line-through; }
	.batches li.now { background: var(--head); color: var(--ink); text-shadow: none; }
	.lock { display: flex; flex-direction: column; gap: 6px; padding: 12px 16px; background: rgb(0 0 0 / 0.35); border-radius: 14px; }
	.lock .btn { flex: none; }
	.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
	.card-foot { margin-top: auto; padding-top: 8px; display: flex; }
	.card-foot .btn { width: 100%; white-space: normal; }
	/* Not room for three: the trip across the top, the other two under it. */
	@media (max-width: 960px) {
		.ticket.laddered { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
		.ticket.laddered .trip { grid-column: 1 / -1; }
	}
	@media (max-width: 820px) {
		.ticket, .ticket.laddered { grid-template-columns: 1fr; padding-inline: 18px; }
		/* Inside the page's narrow gutter. */
		.sticker { right: -10px; }
	}
</style>
