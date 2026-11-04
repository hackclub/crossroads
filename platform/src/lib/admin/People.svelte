<script>
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';
	import { session, refresh } from '../session.svelte.js';

	let q = $state('');
	let users = $state([]);
	let boxTypes = $state([]);
	let stampTypes = $state([]);
	let selected = $state(null);
	let msg = $state(null);
	let adjust = $state({ amount: 0, note: '' });
	let boxTypeId = $state('');
	let stampTypeId = $state('');

	async function search(e) {
		e?.preventDefault();
		try {
			users = await get('/admin/users' + (q ? `?q=${encodeURIComponent(q)}` : ''));
		} catch (err) {
			msg = { tone: 'warn', text: err.message };
		}
	}
	onMount(async () => {
		search();
		[boxTypes, stampTypes] = await Promise.all([get('/admin/box-types'), get('/admin/special-stamp-types')]);
	});

	async function run(fn, ok) {
		msg = null;
		try {
			const res = await fn();
			msg = { tone: 'good', text: ok(res) };
			await search();
			if (selected) selected = users.find((u) => u.id === selected.id) ?? selected;
			if (selected?.id === session.me.id) refresh();
		} catch (err) {
			msg = { tone: 'warn', text: err.message };
		}
	}

	const setRole = (u, role) => run(() => api('PATCH', `/admin/users/${u.id}`, { role }), (r) => `${r.name} is now ${r.role}.`);
	const doAdjust = (e) => {
		e.preventDefault();
		run(() => api('POST', `/admin/users/${selected.id}/stamp-adjustments`, { amount: Number(adjust.amount), note: adjust.note }), (r) => `${r.name} now has ${r.stamps} stamps.`).then(() => (adjust = { amount: 0, note: '' }));
	};
	const giveBox = () => run(() => api('POST', `/admin/users/${selected.id}/boxes`, { box_type_id: Number(boxTypeId) }), () => `Gave ${selected.name} a box.`);
	const giveStamp = () => run(() => api('POST', `/admin/users/${selected.id}/special-stamps`, { special_stamp_type_id: Number(stampTypeId) }), (s) => `Gave ${selected.name} a ${s.type.name}.`);
</script>

<div class="stack">
	<form class="row" onsubmit={search}>
		<input type="search" bind:value={q} placeholder="Name, email or Slack ID" style="max-width: 320px" />
		<button class="btn">Search</button>
		<span class="small muted">Without a search: the newest 50.</span>
	</form>
	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}

	<div class="card table-wrap">
		<table>
			<thead><tr><th>Name</th><th>Email</th><th>Slack</th><th>Stamps</th><th>Verified</th><th>Role</th><th></th></tr></thead>
			<tbody>
				{#each users as u}
					<tr class:you={selected?.id === u.id}>
						<td>{u.name}</td>
						<td class="small">{u.email}</td>
						<td class="small">{u.slack_id ?? '–'}</td>
						<td>{u.stamps}</td>
						<td class="small">{u.verification_status}{u.ysws_eligible ? ' ✓' : ''}</td>
						<td>
							<select value={u.role} onchange={(e) => setRole(u, e.currentTarget.value)} aria-label="Role for {u.name}" style="padding: 3px 6px">
								<option>participant</option><option>reviewer</option><option>admin</option>
							</select>
						</td>
						<td><button class="btn small ghost" onclick={() => (selected = u)}>Manage</button></td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	{#if selected}
		<section class="card">
			<div class="card-title"><h2>{selected.name}: {selected.stamps} stamps</h2><button class="link-btn small" onclick={() => (selected = null)}>Close</button></div>
			<form class="row" onsubmit={doAdjust}>
				<input type="number" bind:value={adjust.amount} style="max-width: 110px" aria-label="Amount" />
				<input bind:value={adjust.note} placeholder="Why (required)" required style="max-width: 320px" />
				<button class="btn">Adjust stamps</button>
				<span class="small muted">Negative takes stamps away, even below zero.</span>
			</form>
			<div class="row">
				<select bind:value={boxTypeId} style="max-width: 220px" aria-label="Box type">
					<option value="" disabled>Box type…</option>
					{#each boxTypes as b}<option value={b.id}>{b.name}</option>{/each}
				</select>
				<button class="btn" disabled={!boxTypeId} onclick={giveBox}>Give box</button>
				<select bind:value={stampTypeId} style="max-width: 220px" aria-label="Special stamp type">
					<option value="" disabled>Special stamp…</option>
					{#each stampTypes as t}<option value={t.id}>{t.name}</option>{/each}
				</select>
				<button class="btn" disabled={!stampTypeId} onclick={giveStamp}>Give stamp</button>
			</div>
		</section>
	{/if}
</div>
