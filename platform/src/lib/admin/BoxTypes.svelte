<script>
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';

	let boxes = $state([]);
	let stampTypes = $state([]);
	let msg = $state(null);
	let form = $state({ name: '', description: '' });
	let weights = $state({}); // box id → { stamp type id → weight }

	async function load() {
		try {
			[boxes, stampTypes] = await Promise.all([get('/admin/box-types'), get('/admin/special-stamp-types')]);
			weights = Object.fromEntries(boxes.map((b) => [b.id, Object.fromEntries(b.drops.map((d) => [d.special_stamp_type_id, d.weight]))]));
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	}
	onMount(load);

	const typeName = (id) => stampTypes.find((t) => t.id === id)?.name ?? id;

	async function create(e) {
		e.preventDefault();
		msg = null;
		try {
			await api('POST', '/admin/box-types', { name: form.name, description: form.description, image_url: null });
			msg = { tone: 'good', text: `Created ${form.name}. Set its drops before giving it out.` };
			form = { name: '', description: '' };
			await load();
		} catch (err) {
			msg = { tone: 'warn', text: err.message };
		}
	}

	async function saveDrops(box) {
		msg = null;
		const drops = Object.entries(weights[box.id] ?? {})
			.filter(([, w]) => Number(w) > 0)
			.map(([id, w]) => ({ special_stamp_type_id: Number(id), weight: Number(w) }));
		try {
			await api('PUT', `/admin/box-types/${box.id}/drops`, drops);
			msg = { tone: 'good', text: `Saved drops for ${box.name}.` };
			await load();
		} catch (err) {
			msg = { tone: 'warn', text: err.message };
		}
	}
</script>

<div class="stack">
	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}
	{#each boxes as box}
		<section class="card">
			<h2>{box.name}</h2>
			{#if box.description}<span class="small muted">{box.description}</span>{/if}
			<p class="small muted">Weights: a stamp's chance is its weight over the total. Leave 0 to leave it out. At least one has to be an unlimited type.</p>
			<table>
				<thead><tr><th>Stamp type</th><th>Supply</th><th>Weight</th></tr></thead>
				<tbody>
					{#each stampTypes as t}
						<tr>
							<td>{t.name} <span class="small muted">{t.rarity}</span></td>
							<td class="small">{t.supply_cap == null ? 'unlimited' : `${t.issued_count}/${t.supply_cap}`}</td>
							<td><input type="number" min="0" value={weights[box.id]?.[t.id] ?? ''} oninput={(e) => ((weights[box.id] ??= {})[t.id] = e.currentTarget.value)} placeholder="0" style="max-width: 100px; padding: 3px 6px" aria-label="Weight for {t.name}" /></td>
						</tr>
					{/each}
				</tbody>
			</table>
			<div class="row"><button class="btn primary small" onclick={() => saveDrops(box)}>Save drops</button>
				<span class="small muted">Now: {box.drops.length ? box.drops.map((d) => `${typeName(d.special_stamp_type_id)} ${d.weight}`).join(', ') : 'no drops'}</span></div>
		</section>
	{/each}

	<form class="card" onsubmit={create}>
		<h2>New box type</h2>
		<div class="grid-halves">
			<label class="field"><span>Name</span><input required bind:value={form.name} /></label>
			<label class="field"><span>Description</span><input bind:value={form.description} /></label>
		</div>
		<div class="row"><button class="btn primary">Create</button></div>
	</form>
</div>
