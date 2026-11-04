<script>
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';
	import { RARITIES } from '../format.js';

	let types = $state([]);
	let msg = $state(null);
	const blank = () => ({ name: '', description: '', image_url: '', rarity: 'common', effect: 'none', supply_cap: '' });
	let form = $state(blank());
	let editing = $state(null);

	async function load() {
		try {
			types = await get('/admin/special-stamp-types');
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	}
	onMount(load);

	async function save(e) {
		e.preventDefault();
		msg = null;
		const body = { ...form, image_url: form.image_url || null, supply_cap: form.supply_cap === '' ? null : Number(form.supply_cap) };
		try {
			if (editing) await api('PATCH', `/admin/special-stamp-types/${editing}`, body);
			else await api('POST', '/admin/special-stamp-types', body);
			msg = { tone: 'good', text: `Saved ${form.name}.` };
			form = blank();
			editing = null;
			await load();
		} catch (err) {
			msg = { tone: 'warn', text: err.message };
		}
	}

	function edit(t) {
		editing = t.id;
		form = { name: t.name, description: t.description, image_url: t.image_url ?? '', rarity: t.rarity, effect: t.effect, supply_cap: t.supply_cap ?? '' };
	}
</script>

<div class="stack">
	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}
	<div class="card table-wrap">
		<table>
			<thead><tr><th>Name</th><th>Rarity</th><th>Effect</th><th>Supply</th><th>Issued</th><th>Used</th><th></th></tr></thead>
			<tbody>
				{#each types as t}
					<tr>
						<td>{t.name}</td><td>{t.rarity}</td><td>{t.effect}</td>
						<td>{t.supply_cap ?? '∞'}</td><td>{t.issued_count}</td><td>{t.applied_count}</td>
						<td><button class="btn small ghost" onclick={() => edit(t)}>Edit</button></td>
					</tr>
				{:else}
					<tr><td colspan="7" class="muted">None yet. Every box needs at least one unlimited type.</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
	<form class="card" onsubmit={save}>
		<h2>{editing ? `Edit ${form.name}` : 'New special stamp type'}</h2>
		<div class="grid-halves">
			<label class="field"><span>Name</span><input required bind:value={form.name} /></label>
			<label class="field"><span>Rarity</span><select bind:value={form.rarity}>{#each RARITIES as r}<option>{r}</option>{/each}</select></label>
			<label class="field"><span>Effect</span><select bind:value={form.effect}><option value="none">none (collectible)</option><option value="priority_review">priority_review</option></select></label>
			<label class="field"><span>Supply cap <em>empty = unlimited</em></span><input type="number" min="0" bind:value={form.supply_cap} /></label>
			<label class="field"><span>Image link <em>optional</em></span><input type="url" bind:value={form.image_url} /></label>
		</div>
		<label class="field"><span>Description</span><textarea bind:value={form.description}></textarea></label>
		<div class="row">
			<button class="btn primary">{editing ? 'Save' : 'Create'}</button>
			{#if editing}<button type="button" class="btn ghost" onclick={() => { editing = null; form = blank(); }}>Cancel</button>{/if}
		</div>
	</form>
</div>
