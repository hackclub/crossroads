<script>
	import { onMount } from 'svelte';
	import { get } from '#lib/api.js';
	import { date } from '#lib/format.js';

	let orders = $state(null);
	let error = $state('');
	onMount(async () => {
		try {
			orders = await get('/orders');
		} catch (e) {
			error = e.message;
		}
	});
	const address = (a) => (a ? [a.name, a.line_1, a.line_2, a.city, a.state, a.postal_code, a.country].filter(Boolean).join(', ') : '–');
</script>

<div class="crumbs"><a href="/shop">Shop</a> / My orders</div>
<div class="page-head"><h1>My orders</h1></div>

{#if error}<p class="callout warn">{error}</p>{/if}
{#if orders?.length}
	<div class="card table-wrap">
		<table>
			<thead><tr><th>Order</th><th>Item</th><th>Paid</th><th>Ordered</th><th>Status</th><th>Ships to</th></tr></thead>
			<tbody>
				{#each orders as o}
					<tr>
						<td class="muted">#{o.id}</td>
						<td>{o.item.name}</td>
						<td>{o.price}</td>
						<td>{date(o.created_at)}</td>
						<td><span class="badge {o.status === 'fulfilled' ? 'good' : o.status === 'cancelled' ? '' : 'info'}">{o.status}</span></td>
						<td class="small">{address(o.shipping_address)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	<p class="small muted" style="margin-top: 12px">Cancelled orders are refunded. Ask in #crossroads-help on Slack if something's wrong.</p>
{:else if orders}
	<div class="card dashed"><p class="muted">No orders yet. <a href="/shop">Browse the shop</a></p></div>
{/if}
