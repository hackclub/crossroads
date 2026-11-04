<script>
	import { view } from '#lib/state.svelte.js';
	import { getData, ORDER_STATUS, PROGRAM } from '#lib/data.js';
	const d = $derived(getData(view.scenario));
</script>

<div class="crumbs"><a href="/shop">Shop</a> / My orders</div>
<div class="page-head">
	<div class="stack-sm"><h1>My orders</h1><p class="muted">Earned {d.earned} {PROGRAM.currency} · Balance {d.balance}</p></div>
</div>

{#if d.orders.length}
	<div class="card table-wrap">
		<table>
			<thead><tr><th>Order</th><th>Item</th><th>Cost</th><th>Date</th><th>Status</th><th></th></tr></thead>
			<tbody>
				{#each d.orders as o}
					<tr>
						<td class="muted">#{o.id}</td>
						<td>{o.item}</td>
						<td>{o.cost}</td>
						<td>{o.date}</td>
						<td>{ORDER_STATUS[o.status]}</td>
						<td>{#if o.status === 'pending'}<button class="btn small ghost">Cancel</button>{:else if o.status === 'fulfilled'}<span class="small muted">Tracking: XX123</span>{/if}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{:else}
	<div class="card dashed"><p class="muted">No orders yet. <a href="/shop">Browse the shop</a></p></div>
{/if}
