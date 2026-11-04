<script>
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';
	import { hours } from '../format.js';

	let milestones = $state([]);
	let boxes = $state([]);
	let msg = $state(null);
	let form = $state({ name: '', hours: 10, box_type_id: '' });

	async function load() {
		try {
			[milestones, boxes] = await Promise.all([get('/admin/milestones'), get('/admin/box-types')]);
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	}
	onMount(load);

	async function create(e) {
		e.preventDefault();
		msg = null;
		try {
			const r = await api('POST', '/admin/milestones', { name: form.name, approved_seconds: Math.round(form.hours * 3600), box_type_id: Number(form.box_type_id) });
			msg = { tone: 'good', text: `Created ${r.milestone.name}. ${r.boxes_granted} people already past it got the box.` };
			form = { name: '', hours: 10, box_type_id: '' };
			await load();
		} catch (err) {
			msg = { tone: 'warn', text: err.message };
		}
	}
	const boxName = (id) => boxes.find((b) => b.id === id)?.name ?? id;
</script>

<div class="stack">
	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}
	<div class="card table-wrap">
		<table>
			<thead><tr><th>Name</th><th>At approved hours</th><th>Gives</th></tr></thead>
			<tbody>
				{#each milestones as m}
					<tr><td>{m.name}</td><td>{hours(m.approved_seconds)}</td><td>{boxName(m.box_type_id)}</td></tr>
				{:else}
					<tr><td colspan="3" class="muted">No milestones yet.</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
	<p class="small muted">Milestones can't be edited or deleted yet (a backend gap). Anyone already past a new milestone gets its box straight away.</p>
	<form class="card" onsubmit={create}>
		<h2>New milestone</h2>
		<div class="grid-halves">
			<label class="field"><span>Name</span><input required bind:value={form.name} placeholder="First 10 hours" /></label>
			<label class="field"><span>Approved hours</span><input type="number" min="0.01" step="0.01" required bind:value={form.hours} /></label>
			<label class="field"><span>Box type</span>
				<select required bind:value={form.box_type_id}>
					<option value="" disabled>Pick one</option>
					{#each boxes as b}<option value={b.id}>{b.name}</option>{/each}
				</select>
			</label>
		</div>
		<div class="row"><button class="btn primary">Create</button></div>
	</form>
</div>
