<script>
	import ImageDrop from '../ImageDrop.svelte';
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';
	import { money } from '../format.js';
	import { session } from '../session.svelte.js';
	import RegionalPrices from './RegionalPrices.svelte';
	import ShopReview from './ShopReview.svelte';
	import { regionalPrices } from './regional-prices.js';

	let items = $state([]);
	let msg = $state(null);
	const blank = () => ({ kind: 'physical', name: '', description: '', image_url: '', price: 100, stipend_dollars: '', stock: '', max_per_user: '', position: 0, active: true, batch_size: '', batch_step: '', emporium: false, grant_dollars: 5, grant_rate: 6.5, region_rule: 'everywhere', region_countries: '' });
	// Tickets and price locks are one per person; the backend insists.
	const onePerPerson = (kind) => kind === 'ticket' || kind === 'price_lock';
	let form = $state(blank());

	// Where an item can go, by the country of someone's Hack Club address
	// (backend/api/regions.go): codes like US, IN, DE.
	const countryNames = new Intl.DisplayNames(['en'], { type: 'region' });
	const countryName = (code) => {
		try {
			const name = countryNames.of(code);
			return name && name !== code ? name : null;
		} catch {
			return null;
		}
	};
	const parseCountries = (text) => [...new Set(String(text).toUpperCase().split(/[\s,;]+/).filter(Boolean))];
	const EU = ['AT', 'BE', 'BG', 'HR', 'CY', 'CZ', 'DK', 'EE', 'FI', 'FR', 'DE', 'GR', 'HU', 'IE', 'IT', 'LV', 'LT', 'LU', 'MT', 'NL', 'PL', 'PT', 'RO', 'SK', 'SI', 'ES', 'SE'];
	const addEU = () => (form.region_countries = [...new Set([...parseCountries(form.region_countries), ...EU])].join(', '));
	const formCountries = $derived(parseCountries(form.region_countries));
	const unknownCountries = $derived(formCountries.filter((c) => !/^[A-Z]{2}$/.test(c) || !countryName(c)));
	// "Only in US, CA" or "Excluded from IN", for the items table.
	// All 27 EU countries read as "the EU"; long lists as a count (the full list is in the tooltip).
	function regionNote(r) {
		if (!r || r.rule === 'everywhere') return '';
		let names = r.countries;
		if (EU.every((c) => names.includes(c))) names = ['the EU', ...names.filter((c) => !EU.includes(c))];
		const list = names.length > 6 ? `${r.countries.length} countries` : names.join(', ');
		return `${r.rule === 'only' ? 'Only in' : 'Excluded from'} ${list}`;
	}
	let editing = $state(null); // item id
	let opened = $state(null); // the item whose regional price research is showing

	async function load() {
		try {
			items = await get('/admin/shop-items');
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	}
	onMount(load);

	const num = (v) => (v === '' || v == null ? null : Number(v));

	function toBody(f) {
		return {
			kind: f.kind,
			name: f.name,
			description: f.description,
			image_url: f.image_url || null,
			price: Number(f.price),
			stipend_cents: f.kind === 'stipend' ? Math.round(Number(f.stipend_dollars) * 100) : null,
			stock: num(f.stock),
			max_per_user: onePerPerson(f.kind) ? 1 : num(f.max_per_user),
			active: f.active,
			position: Number(f.position) || 0,
			batch_size: f.kind === 'ticket' ? num(f.batch_size) : null,
			batch_step: f.kind === 'ticket' ? num(f.batch_step) : null,
			// Grants are always in the Emporium, priced from their rate.
			emporium: f.kind === 'grant' || (!onePerPerson(f.kind) && f.emporium),
			grant_cents: f.kind === 'grant' ? Math.round(Number(f.grant_dollars) * 100) : null,
			grant_cents_per_hour: f.kind === 'grant' ? Math.round(Number(f.grant_rate) * 100) : null,
			// Rail passes and price locks are for everyone.
			regions: onePerPerson(f.kind) ? undefined : f.region_rule === 'everywhere' ? { rule: 'everywhere', countries: [] } : { rule: f.region_rule, countries: parseCountries(f.region_countries) }
		};
	}

	async function save(e) {
		e.preventDefault();
		msg = null;
		try {
			const body = toBody(form);
			if (editing) {
				delete body.kind; // can't change kind after creating
				await api('PATCH', `/admin/shop-items/${editing}`, body);
			} else {
				await api('POST', '/admin/shop-items', body);
			}
			msg = { tone: 'good', text: `Saved ${form.name}.` };
			form = blank();
			editing = null;
			await load();
		} catch (err) {
			msg = { tone: 'warn', text: err.message };
		}
	}

	function edit(i) {
		editing = i.id;
		form = {
			...i,
			image_url: i.image_url ?? '',
			stipend_dollars: i.stipend_cents == null ? '' : i.stipend_cents / 100,
			stock: i.stock ?? '',
			max_per_user: i.max_per_user ?? '',
			batch_size: i.batch_size ?? '',
			batch_step: i.batch_step ?? '',
			grant_dollars: i.grant_cents == null ? 5 : i.grant_cents / 100,
			grant_rate: i.grant_cents_per_hour == null ? 6.5 : i.grant_cents_per_hour / 100,
			region_rule: i.regions?.rule ?? 'everywhere',
			region_countries: (i.regions?.countries ?? []).join(', ')
		};
	}

	// "Batch 2 of 5, 4 left at 1950", for a ticket sold in batches.
	function batchNote(i) {
		const b = i.batches;
		if (!b) return '';
		if (i.stock === 0) return 'Every batch sold';
		return `Batch ${b.current + 1} of ${b.prices.length}, ${b.left_in_batch} left at ${b.prices[b.current]}`;
	}
</script>

<div class="stack">
	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}
	<ShopReview {items} onapplied={load} />
	<div class="card table-wrap">
		<table>
			<thead><tr><th>Order</th><th>Name</th><th>Kind</th><th>Price</th><th>Stock</th><th>Per person</th><th>Active</th><th>Regional prices</th><th></th></tr></thead>
			<tbody>
				{#each items as i}
					{@const info = regionalPrices(i, session.me.stamps_per_hour)}
					<tr class:open={opened === i.id}>
						<td>{i.position}</td>
						<td>{i.name}{#if i.emporium} <span class="badge warn">Emporium</span>{/if}{#if regionNote(i.regions)}<div class="small muted" title={i.regions.countries.map((c) => countryName(c) ?? c).join(', ')}>{regionNote(i.regions)}</div>{/if}</td>
						<td>{i.kind === 'ticket' ? 'rail pass' : i.kind}{i.stipend_cents ? ` (${money(i.stipend_cents)})` : ''}{i.grant_cents ? ` (${money(i.grant_cents)} pieces at ${money(i.grant_cents_per_hour)}/h)` : ''}</td>
						<td>{i.price}{i.batch_step ? ` (+${i.batch_step} a batch)` : ''}{#if i.batches}<br /><span class="small muted">{batchNote(i)}</span>{/if}</td>
						<td>{i.stock ?? '∞'}</td>
						<td>{i.max_per_user ?? '∞'}</td>
						<td>{i.active ? 'Yes' : 'No'}</td>
						<td>
							{#if info}
								<button class="btn small ghost prices-btn" aria-expanded={opened === i.id} onclick={() => (opened = opened === i.id ? null : i.id)}>
									{info.grant != null ? 'Same everywhere' : info.rows.length ? 'See prices' : 'Not priced'}
									<span aria-hidden="true">{opened === i.id ? '▴' : '▾'}</span>
								</button>
							{/if}
						</td>
						<td><button class="btn small ghost" onclick={() => edit(i)}>Edit</button></td>
					</tr>
					{#if opened === i.id && info}
						<tr class="prices-row"><td colspan="9"><RegionalPrices item={i} {info} /></td></tr>
					{/if}
				{:else}
					<tr><td colspan="9" class="muted">No items yet. Start with the rail pass (kind rail pass, price 1800, stock 30, 6 per batch, 150 more a batch), then a price lock (150).</td></tr>
				{/each}
			</tbody>
		</table>
	</div>

	<form class="card" onsubmit={save}>
		<h2>{editing ? `Edit ${form.name}` : 'New item'}</h2>
		<div class="grid-halves">
			<label class="field"><span>Kind</span>
				<select bind:value={form.kind} disabled={!!editing}><option value="ticket">rail pass</option><option value="price_lock">price lock</option><option>physical</option><option>stipend</option><option>grant</option></select>
			</label>
			<label class="field"><span>Name</span><input required bind:value={form.name} /></label>
			{#if form.kind === 'grant'}
				<label class="field"><span>Each piece ($)</span><input type="number" min="0.01" step="0.01" required bind:value={form.grant_dollars} /></label>
				<label class="field"><span>Per approved hour ($) <em>sets the price</em></span><input type="number" min="0.01" step="0.01" required bind:value={form.grant_rate} /></label>
			{:else}
				<label class="field"><span>Price (tickets) {#if form.kind === 'ticket'}<em>the first batch</em>{/if}</span><input type="number" min="0" required bind:value={form.price} /></label>
			{/if}
			{#if form.kind === 'stipend'}
				<label class="field"><span>Stipend ($)</span><input type="number" min="0" step="0.01" required bind:value={form.stipend_dollars} /></label>
			{/if}
			<label class="field"><span>Stock <em>{form.kind === 'grant' ? 'in pieces: the budget; empty = unlimited' : 'empty = unlimited'}</em></span><input type="number" min="0" bind:value={form.stock} /></label>
			{#if form.kind === 'ticket'}
				<label class="field"><span>Passes per batch <em>empty = one price</em></span><input type="number" min="1" bind:value={form.batch_size} /></label>
				<label class="field"><span>Each batch costs more by <em>tickets</em></span><input type="number" min="1" required={num(form.batch_size) != null} bind:value={form.batch_step} /></label>
			{/if}
			{#if !onePerPerson(form.kind) && form.kind !== 'grant'}
				<label class="field"><span>Max per person <em>empty = unlimited</em></span><input type="number" min="1" bind:value={form.max_per_user} /></label>
			{/if}
			<label class="field"><span>Order in shop <em>lower shows first</em></span><input type="number" step="1" bind:value={form.position} /></label>
		</div>
		<div class="field">
			<span>Image <em>optional</em></span>
			<ImageDrop id="item-image" label="Drop an image here" bind:value={form.image_url} />
			<input type="url" aria-label="Image link" placeholder="or paste an image link" bind:value={form.image_url} />
		</div>
		<label class="field"><span>Description</span><textarea bind:value={form.description}></textarea></label>
		{#if !onePerPerson(form.kind)}
			<fieldset class="where">
				<legend>Where it can go <em>by the country of someone's Hack Club address</em></legend>
				<div class="where-row">
					<select bind:value={form.region_rule} aria-label="Where it can go">
						<option value="everywhere">Everywhere</option>
						<option value="only">Only available in…</option>
						<option value="except">Excluded from…</option>
					</select>
					{#if form.region_rule !== 'everywhere'}
						<input type="text" required aria-label="Countries" placeholder="Country codes, like US, CA" bind:value={form.region_countries} />
						<button type="button" class="btn small ghost" onclick={addEU}>Add the EU</button>
					{/if}
				</div>
				{#if form.region_rule !== 'everywhere'}
					{#if formCountries.length}
						<p class="small">
							{form.region_rule === 'only' ? 'Only people in' : 'Hidden from people in'}
							{formCountries.map((c) => countryName(c) ?? c).join(', ')}{form.region_rule === 'only' ? ' see it.' : '.'}
						</p>
					{/if}
					{#if unknownCountries.length}<p class="small callout warn">Not country codes: {unknownCountries.join(', ')}. Use two letters, like US, IN or DE.</p>{/if}
					<p class="small muted">People with no address from Hack Club Auth still see it. A physical item also can't be sent to an address typed at checkout whose country is a code it can't go to.</p>
				{/if}
			</fieldset>
		{/if}
		<label class="check"><input type="checkbox" bind:checked={form.active} /> In the shop (active)</label>
		{#if form.kind === 'grant'}
			<p class="small muted">Grants are only sold in the Smugglers' Emporium. People choose how many pieces; at {session.me.stamps_per_hour} tickets an hour a piece costs {Math.ceil((Number(form.grant_dollars) * session.me.stamps_per_hour) / Number(form.grant_rate)) || '–'} tickets.</p>
		{:else if !onePerPerson(form.kind)}
			<label class="check"><input type="checkbox" bind:checked={form.emporium} /> In the Smugglers' Emporium instead of the shop</label>
		{/if}
		<div class="row">
			<button class="btn primary">{editing ? 'Save' : 'Create item'}</button>
			{#if editing}<button type="button" class="btn ghost" onclick={() => { editing = null; form = blank(); }}>Cancel</button>{/if}
		</div>
	</form>
</div>

<style>
	.prices-btn { white-space: nowrap; }
	tr.open > td, .prices-row > td { background: var(--deep); }
	.prices-row > td { padding: 0 16px 12px; }
	.where { border: 0; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 8px; }
	.where legend { padding: 0; margin-bottom: 6px; }
	.where legend em { color: var(--muted); font-style: normal; font-size: 0.85em; }
	.where-row { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
	.where-row select { width: auto; }
	.where-row input { flex: 1 1 14em; width: auto; }
	.where p { margin: 0; }
</style>
