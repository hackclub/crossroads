<script>
	import ImageDrop from '../ImageDrop.svelte';
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';
	import { money } from '../format.js';

	let items = $state([]);
	let msg = $state(null);
	const blank = () => ({ kind: 'physical', name: '', description: '', image_url: '', price: 100, stipend_dollars: '', stock: '', max_per_user: '', position: 0, active: true, batch_size: '', batch_step: '' });
	// Tickets and price locks are one per person; the backend insists.
	const onePerPerson = (kind) => kind === 'ticket' || kind === 'price_lock';
	let form = $state(blank());
	let editing = $state(null); // item id

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
			batch_step: f.kind === 'ticket' ? num(f.batch_step) : null
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
			batch_step: i.batch_step ?? ''
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
	<div class="card table-wrap">
		<table>
			<thead><tr><th>Order</th><th>Name</th><th>Kind</th><th>Price</th><th>Stock</th><th>Per person</th><th>Active</th><th></th></tr></thead>
			<tbody>
				{#each items as i}
					<tr>
						<td>{i.position}</td>
						<td>{i.name}</td>
						<td>{i.kind}{i.stipend_cents ? ` (${money(i.stipend_cents)})` : ''}</td>
						<td>{i.price}{i.batch_step ? ` (+${i.batch_step} a batch)` : ''}{#if i.batches}<br /><span class="small muted">{batchNote(i)}</span>{/if}</td>
						<td>{i.stock ?? '∞'}</td>
						<td>{i.max_per_user ?? '∞'}</td>
						<td>{i.active ? 'Yes' : 'No'}</td>
						<td><button class="btn small ghost" onclick={() => edit(i)}>Edit</button></td>
					</tr>
				{:else}
					<tr><td colspan="7" class="muted">No items yet. Start with the ticket (kind ticket, price 1800, stock 30, 6 per batch, 150 more a batch), then a price lock (150).</td></tr>
				{/each}
			</tbody>
		</table>
	</div>

	<form class="card" onsubmit={save}>
		<h2>{editing ? `Edit ${form.name}` : 'New item'}</h2>
		<div class="grid-halves">
			<label class="field"><span>Kind</span>
				<select bind:value={form.kind} disabled={!!editing}><option>ticket</option><option value="price_lock">price lock</option><option>physical</option><option>stipend</option></select>
			</label>
			<label class="field"><span>Name</span><input required bind:value={form.name} /></label>
			<label class="field"><span>Price (stamps) {#if form.kind === 'ticket'}<em>the first batch</em>{/if}</span><input type="number" min="0" required bind:value={form.price} /></label>
			{#if form.kind === 'stipend'}
				<label class="field"><span>Stipend ($)</span><input type="number" min="0" step="0.01" required bind:value={form.stipend_dollars} /></label>
			{/if}
			<label class="field"><span>Stock <em>empty = unlimited</em></span><input type="number" min="0" bind:value={form.stock} /></label>
			{#if form.kind === 'ticket'}
				<label class="field"><span>Tickets per batch <em>empty = one price</em></span><input type="number" min="1" bind:value={form.batch_size} /></label>
				<label class="field"><span>Each batch costs more by <em>stamps</em></span><input type="number" min="1" required={form.batch_size !== ''} bind:value={form.batch_step} /></label>
			{/if}
			{#if !onePerPerson(form.kind)}
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
		<label class="check"><input type="checkbox" bind:checked={form.active} /> In the shop (active)</label>
		<div class="row">
			<button class="btn primary">{editing ? 'Save' : 'Create item'}</button>
			{#if editing}<button type="button" class="btn ghost" onclick={() => { editing = null; form = blank(); }}>Cancel</button>{/if}
		</div>
	</form>
</div>
