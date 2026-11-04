<script>
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';
	import { session, refresh } from '#lib/session.svelte.js';
	import { money, stampsPerHour } from '#lib/format.js';

	let items = $state([]);
	let error = $state('');
	let message = $state(null); // { tone, text }
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

	const rate = $derived(stampsPerHour(items.find((i) => i.kind === 'ticket')));
	const kindLabel = { ticket: 'Event ticket', physical: 'Shipped to you', stipend: 'Travel stipend' };

	function soldOutReason(item) {
		if (item.max_per_user != null && item.bought >= item.max_per_user) return item.kind === 'ticket' ? 'You have it' : 'You have the most you can get';
		if (item.stock === 0) return 'Sold out';
		return '';
	}

	async function buy(item) {
		const ok = confirm(`Buy ${item.name} for ${item.price} stamps?`);
		if (!ok) return;
		message = null;
		buying = item.id;
		try {
			const body = item.kind === 'physical' && custom?.itemId === item.id ? { shipping_address: custom.address } : undefined;
			const order = await api('POST', `/shop/items/${item.id}/orders`, body);
			message = { tone: 'good', text: `Ordered ${item.name} (order #${order.id}). ${item.kind === 'ticket' ? "You've qualified for the trip!" : 'Track it in My orders.'}` };
			custom = null;
			await Promise.all([load(), refresh()]);
		} catch (e) {
			const verify = e.status === 403 ? ' Verify your identity at https://auth.hackclub.com, then try again.' : '';
			message = { tone: 'warn', text: e.message + verify };
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
			<p class="muted">You have <strong>{session.me.stamps}</strong> stamps. You earn {rate} per approved hour.</p>
		</div>
		<a class="btn ghost" href="/shop/orders">My orders</a>
	</div>

	{#if error}<p class="callout warn">{error}</p>{/if}
	{#if message}<p class="callout {message.tone}">{message.text}</p>{/if}

	<div class="grid">
		{#each items as item}
			{@const reason = soldOutReason(item)}
			{@const short = item.price - session.me.stamps}
			<div class="card">
				{#if item.image_url}<img src={item.image_url} alt="" style="aspect-ratio: 4/3; object-fit: cover; border-radius: 6px" />{/if}
				<span class="small muted">{kindLabel[item.kind]}{#if item.stipend_cents} · {money(item.stipend_cents)}{/if}</span>
				<strong>{item.name}</strong>
				{#if item.description}<span class="small">{item.description}</span>{/if}
				<div class="row between small">
					<span><strong>{item.price}</strong> stamps</span>
					<span class="muted">{item.stock == null ? '' : `${item.stock} left`}</span>
				</div>
				{#if item.kind === 'physical' && !reason}
					{#if custom?.itemId === item.id}
						<div class="stack-sm small">
							{#each [['name', 'Name'], ['line_1', 'Address line 1'], ['line_2', 'Address line 2'], ['city', 'City'], ['state', 'State / region'], ['postal_code', 'Postal code'], ['country', 'Country']] as [k, label]}
								<input placeholder={label} aria-label={label} bind:value={custom.address[k]} />
							{/each}
							<button class="link-btn" onclick={() => (custom = null)}>Use my Hack Club address instead</button>
						</div>
					{:else}
						<span class="small muted">Ships to the address on your Hack Club account. <button class="link-btn" onclick={() => (custom = { itemId: item.id, address: blank() })}>Use another address</button></span>
					{/if}
				{/if}
				<!-- Pinned to the bottom so the buttons line up across a row of cards. -->
				<div class="card-foot">
					{#if reason}
						<button class="btn" disabled>{reason}</button>
					{:else if short > 0}
						<button class="btn" disabled>Need {short} more stamps (~{Math.ceil(short / rate)}h)</button>
					{:else}
						<button class="btn primary" disabled={buying === item.id} onclick={() => buy(item)}>{buying === item.id ? 'Buying…' : `Buy for ${item.price}`}</button>
					{/if}
				</div>
			</div>
		{:else}
			<p class="muted">Nothing in the shop yet.</p>
		{/each}
	</div>
</div>

<style>
	.card-foot { margin-top: auto; padding-top: 8px; display: flex; }
	.card-foot .btn { width: 100%; white-space: normal; }
</style>
