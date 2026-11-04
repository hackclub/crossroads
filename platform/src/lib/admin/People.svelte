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
	// Role changes are drafted here and only saved with the Save button.
	let roleDraft = $state({}); // user id → role

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

	function setRole(u) {
		const role = roleDraft[u.id];
		const self = u.id === session.me.id;
		const warning = self && role !== 'admin' ? '\n\nThis is you: you will lose access to Admin.' : '';
		if (!confirm(`Change ${u.name} from ${u.role} to ${role}?${warning}`)) return;
		run(() => api('PATCH', `/admin/users/${u.id}`, { role }), (r) => `${r.name} is now ${r.role}.`).then(() => delete roleDraft[u.id]);
	}
	const doAdjust = (e) => {
		e.preventDefault();
		run(() => api('POST', `/admin/users/${selected.id}/stamp-adjustments`, { amount: Number(adjust.amount), note: adjust.note }), (r) => `${r.name} now has ${r.stamps} stamps.`).then(() => (adjust = { amount: 0, note: '' }));
	};
	const giveBox = () => run(() => api('POST', `/admin/users/${selected.id}/boxes`, { box_type_id: Number(boxTypeId) }), () => `Gave ${selected.name} a box.`);
	const giveStamp = () => run(() => api('POST', `/admin/users/${selected.id}/special-stamps`, { special_stamp_type_id: Number(stampTypeId) }), (s) => `Gave ${selected.name} a ${s.type.name}.`);
</script>

<div class="stack">
	<form class="row" onsubmit={search}>
		<input type="search" bind:value={q} placeholder="Name, email or Slack ID" aria-label="Search people" style="max-width: 320px" />
		<button class="btn">Search</button>
		<span class="small muted">Without a search: the newest 50.</span>
	</form>
	<div role="status" aria-live="polite">{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}</div>

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
							<div class="row" style="flex-wrap: nowrap; gap: 6px">
								<select value={roleDraft[u.id] ?? u.role} onchange={(e) => (roleDraft[u.id] = e.currentTarget.value)} aria-label="Role for {u.name}" style="padding: 3px 6px">
									<option>participant</option><option>reviewer</option><option>admin</option>
								</select>
								{#if roleDraft[u.id] && roleDraft[u.id] !== u.role}
									<button class="btn small primary" onclick={() => setRole(u)}>Save</button>
									<button class="btn small ghost" onclick={() => delete roleDraft[u.id]}>Undo</button>
								{/if}
							</div>
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
