<script>
	// The Smugglers' Emporium: a second shop for reviewers, admins, and people
	// an admin let in for a project worth it (GET /emporium/items answers 404
	// to anyone else). Grants are sold here in pieces, priced per approved hour.
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { session, refresh } from '#lib/session.svelte.js';
	import { stampHours, homeCountry } from '#lib/format.js';
	import StampIcon from '#lib/StampIcon.svelte';

	let items = $state(null);
	let ticket = $state(null); // the rail pass, from the shop, to warn before spending what it needs
	let error = $state('');
	let message = $state(null); // { tone, text, verify? }
	let dialog;
	let pending = $state(null); // { item, pieces }
	let other = $state(null); // a different address for physical items; null ships to the Hack Club one
	let dialogError = $state('');
	let buying = $state(false);
	let pieces = $state({}); // grant id -> pieces chosen
	const blank = () => ({ name: '', line_1: '', line_2: '', city: '', state: '', postal_code: '', country: '' });
	const ADDRESS_FIELDS = [['name', 'Name'], ['line_1', 'Address line 1'], ['line_2', 'Address line 2 (optional)'], ['city', 'City'], ['state', 'State / region'], ['postal_code', 'Postal code'], ['country', 'Country']];

	async function load() {
		try {
			items = await get('/emporium/items');
			for (const g of items.filter((i) => i.kind === 'grant')) pieces[g.id] ??= 1;
		} catch (e) {
			error = e.status === 404 ? 'closed' : e.message;
			return;
		}
		// The rail pass is still the point: the buy dialog warns, as the shop's
		// does, when this would leave them short of it. Not having it is fine.
		get('/shop/items').then((all) => (ticket = all.find((i) => i.kind === 'ticket') ?? null)).catch(() => {});
	}
	onMount(load);

	const stamps = $derived(session.me.stamps);
	const rate = $derived(session.me.stamps_per_hour);
	const grants = $derived((items ?? []).filter((i) => i.kind === 'grant'));
	const goods = $derived((items ?? []).filter((i) => i.kind !== 'grant'));
	// When every grant pays the same, the rate is said once, over them all.
	const sharedRate = $derived(
		grants.length > 0 && grants.every((g) => g.grant_cents_per_hour === grants[0].grant_cents_per_hour && g.grant_cents === grants[0].grant_cents)
			? grants[0]
			: null
	);
	const hoursFor = (n) => Math.ceil(n / rate);
	// What buying would leave them short for the rail pass, if it would.
	const ticketAfter = (price) => {
		if (!ticket || ticket.bought > 0) return null;
		const after = ticket.price - (stamps - price);
		return after > 0 ? after : null;
	};
	const dollars = (cents) =>
		'$' + (cents / 100).toLocaleString('en-US', { minimumFractionDigits: cents % 100 ? 2 : 0, maximumFractionDigits: 2 });

	// What n pieces of a grant cost: the stamps for the approved hours they're
	// worth, rounded up, as the backend charges (api/shop.go grantPrice).
	const grantPrice = (g, n) => Math.ceil((n * g.grant_cents * rate) / g.grant_cents_per_hour);
	// The most pieces they could get now: what they can afford, what's left, and 200 at once.
	function mostPieces(g) {
		let n = Math.min(200, g.stock ?? 200);
		while (n > 0 && grantPrice(g, n) > stamps) n--;
		return n;
	}
	function setPieces(g, n) {
		pieces[g.id] = Math.max(1, Math.min(200, g.stock ?? 200, Math.round(Number(n) || 1)));
	}

	function soldOutReason(item) {
		if (item.max_per_user != null && item.bought >= item.max_per_user) return 'You have the most you can get';
		if (item.stock === 0) return 'All gone';
		return '';
	}

	function ask(item, n = 1) {
		pending = { item, pieces: n, price: item.kind === 'grant' ? grantPrice(item, n) : item.price };
		other = item.kind === 'physical' && !session.me.address ? blank() : null;
		dialogError = '';
		message = null;
		dialog.showModal();
	}

	async function buy(e) {
		e.preventDefault();
		const { item, pieces: n } = pending;
		dialogError = '';
		buying = true;
		try {
			const body = item.kind === 'grant' ? { quantity: n } : item.kind === 'physical' && other ? { shipping_address: other } : undefined;
			const order = await api('POST', `/shop/items/${item.id}/orders`, body);
			message = {
				tone: 'good',
				text:
					item.kind === 'grant'
						? `Done: a ${dollars(order.grant_cents)} grant (order #${order.id}). We'll be in touch about getting it to you. Track it in My orders.`
						: `Ordered ${item.name} (order #${order.id}). Track it in My orders.`
			};
			dialog.close();
			await Promise.all([load(), refresh()]);
		} catch (err) {
			if (err.status === 422) dialogError = err.message;
			else {
				dialog.close();
				message = { tone: 'warn', text: err.message, verify: err.status === 403 };
				if (err.status === 409 || err.status === 404) await load();
			}
		} finally {
			buying = false;
		}
	}
	const lines = (a) => [a.name, a.line_1, a.line_2, [a.city, a.state, a.postal_code].filter(Boolean).join(', '), a.country].filter(Boolean);
</script>

<svelte:head><title>Smugglers' Emporium · Crossroads</title></svelte:head>

<div class="emporium">
	{#if error === 'closed'}
		<div class="closed">
			<p class="whisper">Nothing to see here</p>
			<p class="muted">Move along.</p>
		</div>
	{:else}
		<header class="sign">
			<div class="sign-text">
				<h1>The Smugglers' Emporium</h1>
				<p class="lede">
					<span class="hush">Keep this between us.</span> Goods that don't make it to the shop window, for reviewers and for makers whose
					projects caught our eye.
					{#if session.me.role === 'participant'}You're here because of something you built.{/if}
				</p>
				<p class="purse">You have <b>{stamps}</b><StampIcon /> tickets, worth {stampHours(stamps, rate)} of approved time.</p>
				{#if homeCountry(session.me)}<p class="small muted">Showing what can be sent to {homeCountry(session.me)}, where your Hack Club address is.</p>{/if}
			</div>
			<a class="btn ghost orders" href="/shop/orders">My orders</a>
		</header>

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

		{#if grants.length}
		<section class="ledger" aria-labelledby="grants-title">
			<div class="ledger-head">
				<h2 id="grants-title">Grants</h2>
				{#if sharedRate}<span class="rate"><b>{dollars(sharedRate.grant_cents_per_hour)}</b> for every approved hour, in {dollars(sharedRate.grant_cents)} pieces</span>{/if}
			</div>
			<ul>
		{#each grants as g (g.id)}
			{@const n = pieces[g.id] ?? 1}
			{@const cost = grantPrice(g, n)}
			{@const reason = soldOutReason(g)}
			<li class="grant" aria-labelledby="grant-{g.id}">
				<div class="grant-text">
					<h3 id="grant-{g.id}">{g.name}</h3>
					{#if !sharedRate}<p class="rate"><b>{dollars(g.grant_cents_per_hour)}</b> for every approved hour, in {dollars(g.grant_cents)} pieces.</p>{/if}
					{#if g.description}<p class="muted">{g.description}</p>{/if}
					{#if g.stock != null}<p class="small muted">{dollars(g.stock * g.grant_cents)} left in the pot.</p>{/if}
				</div>
				<div class="grant-buy">
					{#if reason}
						<button class="btn" disabled>{reason}</button>
					{:else}
						<div class="stepper" role="group" aria-label="Grant amount">
							<button class="step" aria-label="{dollars(g.grant_cents)} less" disabled={n <= 1} onclick={() => setPieces(g, n - 1)}>
								<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h12" /></svg>
							</button>
							<label class="amount">
								<span class="visually-hidden">Grant, in dollars</span>
								<span aria-hidden="true">$</span>
								<input type="number" min={g.grant_cents / 100} step={g.grant_cents / 100} value={(n * g.grant_cents) / 100}
									onchange={(e) => setPieces(g, Number(e.currentTarget.value) / (g.grant_cents / 100))} />
							</label>
							<button class="step" aria-label="{dollars(g.grant_cents)} more" disabled={n >= Math.min(200, g.stock ?? 200)} onclick={() => setPieces(g, n + 1)}>
								<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h12M10 4v12" /></svg>
							</button>
						</div>
						<p class="cost">{cost} tickets <span class="muted">· {stampHours(cost, rate)}</span></p>
						{#if cost > stamps}
							<button class="btn" disabled>Need {cost - stamps} more tickets (~{hoursFor(cost - stamps)}h)</button>
							{#if mostPieces(g) > 0}<button class="link-btn small" onclick={() => setPieces(g, mostPieces(g))}>Most you can get now: {dollars(mostPieces(g) * g.grant_cents)}</button>{/if}
						{:else}
							<button class="btn primary" onclick={() => ask(g, n)}>Take {dollars(n * g.grant_cents)}</button>
						{/if}
					{/if}
				</div>
			</li>
		{/each}
			</ul>
		</section>
		{/if}

		{#if goods.length}
			<section class="ledger manifest" aria-labelledby="manifest-title">
				<div class="ledger-head">
					<h2 id="manifest-title">Manifest</h2>
					<span class="small muted">{goods.length} {goods.length === 1 ? 'item' : 'items'} aboard</span>
				</div>
				<ul>
					{#each goods as item (item.id)}
						{@const reason = soldOutReason(item)}
						{@const short = item.price - stamps}
						<li>
							<div class="thumb">{#if item.image_url}<img src={item.image_url} alt="" loading="lazy" />{/if}</div>
							<div class="what">
								<h3>{item.name}</h3>
								{#if item.description}<p class="desc">{item.description}</p>{/if}
							</div>
							<p class="cost-col"><b>{item.price.toLocaleString()}</b> <span>tickets</span></p>
							<div class="act">
								{#if reason}
									<button class="btn" disabled>{reason}</button>
								{:else if short > 0}
									<button class="btn" disabled>Need {short} more <span class="muted">(~{hoursFor(short)}h)</span></button>
								{:else}
									<button class="btn primary" aria-label="Buy {item.name} for {item.price} tickets" onclick={() => ask(item)}>Buy</button>
								{/if}
							</div>
						</li>
					{/each}
				</ul>
			</section>
		{:else if items && !grants.length}
			<p class="muted empty">The shelves are bare tonight. Come back later.</p>
		{/if}
	{/if}
</div>

<dialog bind:this={dialog} aria-labelledby="buy-title" onclose={() => (pending = null)}>
	{#if pending}
		{@const { item } = pending}
		<form class="stack" method="dialog" onsubmit={buy}>
			<h2 id="buy-title">{item.kind === 'grant' ? `Take a ${dollars(pending.pieces * item.grant_cents)} grant?` : `Buy ${item.name}?`}</h2>
			<dl class="sums">
				<dt>Price</dt><dd>{pending.price} tickets</dd>
				<dt>You have</dt><dd>{stamps} tickets</dd>
				<dt>Left after</dt><dd><b>{stamps - pending.price} tickets</b></dd>
			</dl>
			{#if item.kind === 'grant'}
				<p class="small">That's {stampHours(pending.price, rate)} of approved time at {dollars(item.grant_cents_per_hour)} an hour. We'll send it once an admin processes the order.</p>
			{/if}
			{#if ticketAfter(pending.price)}
				{@const short = ticketAfter(pending.price)}
				<p class="callout warn">Your rail pass costs {ticket.price}. After this you'd need {short} more tickets (about {hoursFor(short)} approved hours) to buy it.</p>
			{/if}
			{#if item.kind === 'physical'}
				<div class="stack-sm">
					<span class="ship-label">Ship to</span>
					{#if other}
						<div class="addr-form">
							{#each ADDRESS_FIELDS as [k, label]}
								<input placeholder={label} aria-label={label} required={k !== 'line_2' && k !== 'state' && k !== 'postal_code'} bind:value={other[k]} />
							{/each}
						</div>
						{#if session.me.address}<button type="button" class="link-btn" onclick={() => (other = null)}>Use my Hack Club address instead</button>{/if}
					{:else}
						<address class="addr">{#each lines(session.me.address) as l}<span>{l}</span>{/each}</address>
						<span class="small">The address on your Hack Club account. <button type="button" class="link-btn" onclick={() => (other = blank())}>Use a different address</button></span>
					{/if}
				</div>
			{/if}
			{#if dialogError}<p class="callout warn" role="alert">{dialogError}</p>{/if}
			<div class="row" style="justify-content: flex-end">
				<button type="button" class="btn ghost" onclick={() => dialog.close()}>Cancel</button>
				<button class="btn primary" disabled={buying}>{buying ? 'Working…' : item.kind === 'grant' ? `Take it for ${pending.price}` : `Buy for ${pending.price}`}</button>
			</div>
		</form>
	{/if}
</dialog>

<style>
	/* Lamplight in a back room: a darker veil over the photo, warm brass
	   for what matters, and the shop's own cards and buttons underneath. */
	.emporium {
		--brass: #d9a441;
		--brass-dim: #8a6a2c;
		display: flex;
		flex-direction: column;
		gap: 24px;
		padding: 28px 32px 36px;
		border-radius: var(--radius);
		--rule: rgb(236 225 205 / 0.14); /* the manifest's ruled lines */
		background: rgb(10 8 6 / 0.86);
		box-shadow: inset 0 0 0 1px var(--rule);
	}
	@media (max-width: 600px) { .emporium { padding: 20px 16px 28px; } }
	/* Brass for what the browser draws too: selection and focus. */
	.emporium ::selection, dialog ::selection { background: rgb(217 164 65 / 0.4); color: var(--text); }
	.emporium :focus-visible, dialog :focus-visible { outline: 2px solid var(--brass, #d9a441); outline-offset: 3px; }
	/* The shop's head: the name and what you have on the left, My orders on
	   the right. */
	.sign { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px 24px; }
	.sign-text { display: flex; flex-direction: column; gap: 6px; max-width: 760px; }
	.sign h1 { font-size: clamp(44px, 6vw, 72px); line-height: 1; font-style: italic; text-wrap: balance; }
	.sign p { margin: 0; }
	.lede { color: var(--muted); }
	.hush { color: var(--text); font-style: italic; }
	.orders { flex: none; }
	.status:not(:has(p)) { margin-top: -24px; }
	.purse { font-size: 22px; }
	.purse b { font-weight: 400; }

	/* Grants and goods are both ruled ledgers: a heading over a rule, then
	   a row each. */
	.ledger { display: flex; flex-direction: column; gap: 6px; }
	.ledger-head { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 4px 16px; padding-bottom: 6px; border-bottom: 1px solid var(--rule); }
	.ledger h2 { font-size: 32px; line-height: 1.1; font-style: italic; }
	.ledger ul { list-style: none; margin: 0; padding: 0; }
	.rate { font-size: 20px; }
	.rate b { font-weight: 400; color: var(--brass); }

	/* A grant: what it's for on the left; the amount, its cost and the
	   button on the right. */
	.grant {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 26em);
		gap: 12px 32px;
		align-items: center;
		padding: 16px 0;
		border-bottom: 1px solid var(--rule);
	}
	@media (max-width: 800px) { .grant { grid-template-columns: minmax(0, 1fr); } }
	.grant-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.grant-text p { margin: 0; }
	.grant h3 { font-size: 28px; line-height: 1.1; }
	.grant-text .muted { font-size: 19px; }
	.grant-buy { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); align-items: center; gap: 6px 10px; }
	.grant-buy > .btn, .grant-buy > .link-btn { grid-column: 2; }
	.grant-buy > .btn:only-child { grid-column: 1 / -1; }
	.stepper { display: grid; grid-template-columns: 40px minmax(0, 1fr) 40px; gap: 4px; }
	.step {
		display: grid;
		place-items: center;
		color: var(--text);
		background: var(--deep);
		border: 0;
		border-radius: 10px;
		cursor: pointer;
		transition: background-color 0.15s ease-out;
	}
	.step svg { width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 2.2; stroke-linecap: round; }
	.step:hover:not(:disabled) { background: var(--light); }
	.step:disabled { color: var(--muted); cursor: not-allowed; }
	.amount { display: flex; align-items: center; gap: 2px; padding: 0 10px; border-radius: 10px; background: var(--navy-deep); font-size: 24px; }
	.amount input { width: 100%; padding: 4px 0; font-size: 24px; background: transparent; box-shadow: none; text-align: center; font-variant-numeric: tabular-nums; appearance: textfield; -moz-appearance: textfield; }
	.amount input::-webkit-inner-spin-button, .amount input::-webkit-outer-spin-button { -webkit-appearance: none; margin: 0; }
	.cost { grid-column: 1; grid-row: 2; margin: 0; font-size: 17px; text-align: center; color: var(--muted); font-variant-numeric: tabular-nums; }

	/* The manifest: ruled rows, the price in its own column, the action on
	   the right. Brass only on the prices and the buy buttons. */
	.manifest li {
		display: grid;
		grid-template-columns: 72px minmax(0, 1fr) 7.5em minmax(13em, auto);
		align-items: center;
		gap: 8px 20px;
		padding: 14px 0;
		border-bottom: 1px solid var(--rule);
	}
	.thumb { width: 72px; height: 72px; overflow: hidden; background: var(--navy-deep); border-radius: 6px; }
	.thumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
	.what { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.what h3 { font-size: 26px; line-height: 1.15; }
	.desc {
		margin: 0;
		font-size: 19px;
		color: var(--muted);
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.cost-col { margin: 0; text-align: right; line-height: 1.1; }
	.cost-col b { display: block; font-weight: 400; font-size: 28px; color: var(--brass); font-variant-numeric: tabular-nums; }
	.cost-col span { font-size: 16px; color: var(--muted); }
	.act { display: flex; justify-content: flex-end; }
	.act .btn { width: 100%; white-space: normal; }
	.act .btn .muted { color: inherit; opacity: 0.75; }
	@media (max-width: 720px) {
		.manifest li { grid-template-columns: 64px minmax(0, 1fr) auto; }
		.thumb { width: 64px; height: 64px; }
		.act { grid-column: 1 / -1; }
	}
	.empty { margin: 0; }
	.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
	/* The one thing to do here, in brass. */
	.grant-buy .btn.primary, .act .btn.primary, dialog .btn.primary { background: var(--brass, #d9a441); color: #1a1208; }

	.closed { display: grid; place-items: center; gap: 6px; min-height: 40vh; text-align: center; }
	.closed p { margin: 0; }
	.whisper { font-size: 20px; letter-spacing: 0.32em; text-transform: uppercase; color: #d9a441; }

	dialog {
		width: min(560px, calc(100vw - 32px));
		padding: 24px 28px;
		color: var(--text);
		background: #17110b;
		border: 1px solid var(--brass-dim, #8a6a2c);
		border-radius: var(--radius);
		box-shadow: 0 24px 60px rgb(0 0 0 / 0.7);
	}
	dialog::backdrop { background: rgb(0 0 0 / 0.75); }
	dialog .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); }
	dialog h2 { font-size: 36px; }
	.sums { display: grid; grid-template-columns: auto 1fr; gap: 6px 20px; margin: 0; font-size: 26px; }
	.sums dt { color: var(--muted); }
	.sums dd { margin: 0; text-align: right; }
	.sums b { font-weight: 400; padding: 2px 12px; background: var(--navy); border-radius: 8px; }
	.ship-label { font-size: 26px; }
	.addr { display: flex; flex-direction: column; padding: 12px 16px; font-style: normal; font-size: 23px; background: var(--navy-deep); border-radius: 10px; }
	.addr-form { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
	.addr-form input { font-size: 22px; }
	.addr-form input:nth-child(-n + 3) { grid-column: 1 / -1; }
</style>
