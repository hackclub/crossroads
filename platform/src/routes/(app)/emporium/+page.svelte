<script>
	// The Smugglers' Emporium: a second shop for reviewers, admins, and people
	// an admin let in for a project worth it (GET /emporium/items answers 404
	// to anyone else). Grants are sold here in pieces, priced per approved hour.
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { session, refresh } from '#lib/session.svelte.js';
	import { stampHours } from '#lib/format.js';
	import StampIcon from '#lib/StampIcon.svelte';

	let items = $state(null);
	let error = $state('');
	let message = $state(null); // { tone, text }
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
		}
	}
	onMount(load);

	const stamps = $derived(session.me.stamps);
	const rate = $derived(session.me.stamps_per_hour);
	const grants = $derived((items ?? []).filter((i) => i.kind === 'grant'));
	const goods = $derived((items ?? []).filter((i) => i.kind !== 'grant'));
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
				message = { tone: 'warn', text: err.message };
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
			<p class="kicker">Nothing to see here</p>
			<p class="muted">Move along.</p>
		</div>
	{:else}
		<header class="sign">
			<p class="kicker">Keep this between us</p>
			<h1>The Smugglers' Emporium</h1>
			<p class="muted">
				Goods that don't make it to the shop window, for reviewers and for makers whose projects caught our eye.
				{#if session.me.role === 'participant'}You're here because of something you built.{/if}
			</p>
			<p class="purse">You have <b>{stamps}</b><StampIcon /> stamps, worth {stampHours(stamps, rate)} of approved time.</p>
		</header>

		{#if error}<p class="callout warn">{error}</p>{/if}
		{#if message}<p class="callout {message.tone}" role="status">{message.text}</p>{/if}

		{#each grants as g (g.id)}
			{@const n = pieces[g.id] ?? 1}
			{@const cost = grantPrice(g, n)}
			{@const reason = soldOutReason(g)}
			<section class="grant" aria-labelledby="grant-{g.id}">
				<div class="grant-text">
					<p class="kicker">Grant</p>
					<h2 id="grant-{g.id}">{g.name}</h2>
					<p class="rate"><b>{dollars(g.grant_cents_per_hour)}</b> for every approved hour, in {dollars(g.grant_cents)} pieces.</p>
					{#if g.description}<p class="muted">{g.description}</p>{/if}
					{#if g.stock != null}<p class="small muted">{dollars(g.stock * g.grant_cents)} left in the pot.</p>{/if}
				</div>
				<div class="grant-buy">
					{#if reason}
						<button class="btn" disabled>{reason}</button>
					{:else}
						<div class="stepper" role="group" aria-label="Grant amount">
							<button class="step" aria-label="{dollars(g.grant_cents)} less" disabled={n <= 1} onclick={() => setPieces(g, n - 1)}>−</button>
							<label class="amount">
								<span class="visually-hidden">Grant, in dollars</span>
								<span aria-hidden="true">$</span>
								<input type="number" min={g.grant_cents / 100} step={g.grant_cents / 100} value={(n * g.grant_cents) / 100}
									onchange={(e) => setPieces(g, Number(e.currentTarget.value) / (g.grant_cents / 100))} />
							</label>
							<button class="step" aria-label="{dollars(g.grant_cents)} more" disabled={n >= Math.min(200, g.stock ?? 200)} onclick={() => setPieces(g, n + 1)}>+</button>
						</div>
						<p class="cost">{cost} stamps <span class="muted">· {stampHours(cost, rate)}</span></p>
						{#if cost > stamps}
							<button class="btn" disabled>Need {cost - stamps} more stamps</button>
							{#if mostPieces(g) > 0}<button class="link-btn small" onclick={() => setPieces(g, mostPieces(g))}>Most you can get now: {dollars(mostPieces(g) * g.grant_cents)}</button>{/if}
						{:else}
							<button class="btn primary" onclick={() => ask(g, n)}>Take {dollars(n * g.grant_cents)}</button>
						{/if}
					{/if}
				</div>
			</section>
		{/each}

		{#if goods.length}
			<div class="items">
				{#each goods as item (item.id)}
					{@const reason = soldOutReason(item)}
					{@const short = item.price - stamps}
					<article class="card item">
						<div class="thumb">{#if item.image_url}<img src={item.image_url} alt="" loading="lazy" />{/if}</div>
						<div class="info">
							<h3>{item.name}</h3>
							<span class="price">{item.price} stamps</span>
							{#if item.description}<p class="desc">{item.description}</p>{/if}
						</div>
						<div class="card-foot">
							{#if reason}
								<button class="btn" disabled>{reason}</button>
							{:else if short > 0}
								<button class="btn" disabled>Need {short} more stamps</button>
							{:else}
								<button class="btn primary" onclick={() => ask(item)}>Buy for {item.price}</button>
							{/if}
						</div>
					</article>
				{/each}
			</div>
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
				<dt>Price</dt><dd>{pending.price} stamps</dd>
				<dt>You have</dt><dd>{stamps} stamps</dd>
				<dt>Left after</dt><dd><b>{stamps - pending.price} stamps</b></dd>
			</dl>
			{#if item.kind === 'grant'}
				<p class="small">That's {stampHours(pending.price, rate)} of approved time at {dollars(item.grant_cents_per_hour)} an hour. We'll send it once an admin processes the order.</p>
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
		background:
			radial-gradient(ellipse 70% 55% at 50% 0%, rgb(217 164 65 / 0.16), transparent 70%),
			rgb(8 6 5 / 0.82);
		box-shadow: inset 0 0 0 1px rgb(217 164 65 / 0.18);
	}
	@media (max-width: 600px) { .emporium { padding: 20px 16px 28px; } }
	.kicker {
		margin: 0;
		font-size: 18px;
		letter-spacing: 0.32em;
		text-transform: uppercase;
		color: var(--brass);
	}
	.sign { display: flex; flex-direction: column; gap: 6px; max-width: 760px; }
	.sign h1 { font-size: clamp(44px, 6vw, 72px); line-height: 1; font-style: italic; }
	.sign p { margin: 0; }
	.purse { font-size: 22px; }
	.purse b { font-weight: 400; color: var(--brass); }

	.grant {
		display: grid;
		grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
		gap: 18px 32px;
		align-items: center;
		padding: 22px 26px;
		border-radius: var(--radius);
		background: rgb(30 22 14 / 0.85);
		border: 1px dashed var(--brass-dim);
	}
	@media (max-width: 800px) { .grant { grid-template-columns: minmax(0, 1fr); } }
	.grant-text { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
	.grant-text p { margin: 0; }
	.grant h2 { font-size: 40px; line-height: 1.05; }
	.rate { font-size: 24px; }
	.rate b { font-weight: 400; color: var(--brass); }
	.grant-buy { display: flex; flex-direction: column; align-items: stretch; gap: 10px; }
	.stepper { display: grid; grid-template-columns: 52px minmax(0, 1fr) 52px; gap: 6px; }
	.step {
		font: inherit;
		font-size: 30px;
		line-height: 1;
		color: var(--text);
		background: var(--deep);
		border: 0;
		border-radius: 10px;
		cursor: pointer;
	}
	.step:disabled { color: var(--muted); cursor: not-allowed; }
	.amount { display: flex; align-items: center; gap: 4px; padding: 0 14px; border-radius: 10px; background: var(--navy-deep); font-size: 34px; }
	.amount input { width: 100%; padding: 6px 0; font-size: 34px; background: transparent; box-shadow: none; text-align: center; font-variant-numeric: tabular-nums; }
	.cost { margin: 0; font-size: 24px; text-align: center; font-variant-numeric: tabular-nums; }

	.items { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 16px; }
	.item { display: grid; grid-template-columns: 104px minmax(0, 1fr); grid-template-rows: 1fr auto; gap: 12px 16px; padding: 16px; background: rgb(30 22 14 / 0.85); }
	.thumb { width: 104px; height: 104px; overflow: hidden; background: var(--navy-deep); border-radius: 14px; }
	.thumb img { width: 100%; height: 100%; object-fit: cover; display: block; }
	.info { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.info h3 { font-size: 28px; line-height: 1.1; }
	.price { font-size: 22px; color: var(--brass); }
	.desc { margin: 0; font-size: 20px; color: var(--muted); }
	.item .card-foot { grid-column: 1 / -1; }
	.empty { margin: 0; }
	.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
	/* The one thing to do here, in brass. */
	.grant-buy .btn.primary, .item .btn.primary, dialog .btn.primary { background: var(--brass, #d9a441); color: #1a1208; }

	.closed { display: grid; place-items: center; gap: 6px; min-height: 40vh; text-align: center; }
	.closed p { margin: 0; }

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
