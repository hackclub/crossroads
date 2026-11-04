<script>
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';
	import { date } from '../format.js';

	let status = $state('pending');
	let orders = $state([]);
	let notes = $state({});
	let msg = $state(null);

	async function load() {
		try {
			orders = await get('/admin/orders' + (status ? `?status=${status}` : ''));
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	}
	onMount(load);

	async function set(o, s) {
		if (s === 'cancelled' && !confirm(`Cancel order #${o.id}? ${o.user.name} gets ${o.price} stamps back.`)) return;
		msg = null;
		try {
			await api('PATCH', `/admin/orders/${o.id}`, { status: s, notes: notes[o.id] || undefined });
			msg = { tone: 'good', text: `Order #${o.id} ${s}.` };
			await load();
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	}
	const address = (a) => (a ? [a.name, a.line_1, a.line_2, a.city, a.state, a.postal_code, a.country].filter(Boolean).join(', ') : '–');
</script>

<div class="stack">
	<div class="row">
		<select bind:value={status} onchange={load} style="max-width: 180px" aria-label="Status">
			<option value="pending">Pending</option><option value="fulfilled">Fulfilled</option><option value="cancelled">Cancelled</option><option value="">All</option>
		</select>
	</div>
	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}
	{#if orders.length}
		<div class="card table-wrap">
			<table>
				<thead><tr><th>#</th><th>Who</th><th>Item</th><th>Paid</th><th>Ordered</th><th>Ship to</th><th>Status</th><th></th></tr></thead>
				<tbody>
					{#each orders as o}
						<tr>
							<td>{o.id}</td>
							<td class="small">{o.user.name}<br /><span class="muted">{o.user.email}</span></td>
							<td>{o.item.name}</td>
							<td>{o.price}</td>
							<td class="small">{date(o.created_at)}</td>
							<td class="small">{address(o.shipping_address)}</td>
							<td class="small">{o.status}{o.notes ? ` · ${o.notes}` : ''}</td>
							<td>
								{#if o.status === 'pending'}
									<div class="stack-sm">
										<input class="small" placeholder="Notes" bind:value={notes[o.id]} style="padding: 3px 6px" />
										<div class="row"><button class="btn small primary" onclick={() => set(o, 'fulfilled')}>Fulfilled</button><button class="btn small ghost" onclick={() => set(o, 'cancelled')}>Cancel + refund</button></div>
									</div>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<p class="muted small">No orders.</p>
	{/if}
</div>
