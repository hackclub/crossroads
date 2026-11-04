<script>
	import Img from '#lib/components/Img.svelte';
	import Note from '#lib/components/Note.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, shopItems, PROGRAM } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	const categories = [...new Set(shopItems.map((i) => i.category))];
	let category = $state('');
	let affordable = $state(false);

	const items = $derived(
		shopItems.filter((i) => !category || i.category === category).filter((i) => !affordable || i.cost <= d.balance)
	);
</script>

<div class="page-head">
	<div class="stack-sm">
		<h1>Shop</h1>
		<p class="muted">Spend {PROGRAM.currency} on things for your projects. Spending never affects your trip progress.</p>
	</div>
	<div class="row">
		<div class="card" style="padding: 10px 16px; flex-direction: row; align-items: baseline; gap: 8px">
			<strong class="big">{d.balance}</strong><span class="muted small">{PROGRAM.currency}</span>
		</div>
		<a class="btn ghost" href="/shop/orders">My orders ({d.orders.length})</a>
	</div>
</div>

<div class="tabs">
	<button class:active={!category} onclick={() => (category = '')}>All</button>
	{#each categories as c}<button class:active={category === c} onclick={() => (category = c)}>{c}</button>{/each}
</div>
<label class="check small" style="margin-bottom: 16px"><input type="checkbox" bind:checked={affordable} /> Only show what I can afford</label>

<div class="grid">
	{#each items as item}
		{@const short = item.cost - d.balance}
		<a class="card project-card" href="/shop/{item.id}">
			<Img label={item.name} ratio="4 / 3" />
			<div class="row between"><strong>{item.name}</strong></div>
			<div class="row between small">
				<span><strong>{item.cost}</strong> {PROGRAM.currency}</span>
				<span class="muted">{item.stock == null ? 'Unlimited' : `${item.stock} left`}</span>
			</div>
			<span class="small muted">{short > 0 ? `${short} more to go · about ${Math.ceil(short / PROGRAM.tokensPerHour)}h of building` : 'You can afford this'}</span>
		</a>
	{/each}
</div>

<div style="margin-top: 24px" class="stack">
	<div class="card dashed row between">
		<span class="small">Want something that isn't here?</span>
		<button class="btn small ghost">Suggest an item</button>
	</div>
	<Note from="Macondo / Beest">"Time to earn" on every item makes the shop a goal, not just a store. Items, prices and currency name are placeholders.</Note>
</div>
