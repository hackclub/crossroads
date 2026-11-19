<script>
	// One shop item's regional price research, opened from its row on the
	// admin Shop items page: where it's bought in each region, what it costs
	// delivered, and what that comes to per hour of work (regional-prices.js).
	// Research only: the shop charges everyone the same.
	import { REGIONS } from './regional-prices.js';

	let { item, info } = $props();

	const usd = (n) => (n == null ? '–' : `$${n.toFixed(2)}`);
	const hrs = (h) => `${Number.isInteger(h) ? h : h.toFixed(1)}h`;
	const local = (r) => {
		if (r.price_local == null || r.currency === 'USD') return '';
		const parts = [`${r.price_local.toLocaleString()} ${r.currency}`];
		if (r.shipping_local) parts.push(`+ ${r.shipping_local.toLocaleString()} shipping`);
		if (r.import_fees_local) parts.push(`+ ${r.import_fees_local.toLocaleString()} import fees`);
		return parts.join(' ');
	};
</script>

<div class="prices">
	<p class="small">
		{item.price} tickets is {hrs(info.hours)} of work.
		{#if info.grant != null}
			A ${(info.grant / 100).toLocaleString()} card grant{item.kind === 'grant' ? ' piece' : ''} is worth the same everywhere: {usd(info.perHour)} for each hour of work.
		{:else if info.cheapest}
			Cheapest in {info.cheapest.label} ({usd(info.cheapest.total_usd)} delivered), most expensive in {info.priciest.label} ({usd(info.priciest.total_usd)}){info.missing ? `; no price for ${info.missing} region${info.missing === 1 ? '' : 's'}` : ''}.
		{/if}
	</p>

	{#if info.grant != null}
		<!-- One sentence says it all. -->
	{:else if !info.rows.length}
		<p class="small muted">No regional prices for this item yet. Add it to <code>platform/src/lib/admin/regional-prices.json</code>.</p>
	{:else}
		<div class="table-wrap">
			<table>
				<thead><tr><th>Region</th><th>Where</th><th class="num">Delivered</th><th class="num">Per hour of work</th><th>Notes</th></tr></thead>
				<tbody>
					{#each info.rows as r (r.code)}
						<tr>
							<td>{REGIONS[r.code]}</td>
							<td>
								{#if r.url}<a href={r.url} target="_blank" rel="noopener noreferrer">{r.store || 'link'}</a>{:else}{r.store || '–'}{/if}
								{#if local(r)}<div class="tiny muted">{local(r)}</div>{/if}
							</td>
							<td class="num">{usd(r.total_usd)}{#if r.total_usd != null && !r.complete}<div class="tiny muted">+ unknown shipping</div>{/if}</td>
							<td class="num">{usd(r.perHour)}</td>
							<td class="notes small">{#if r.in_stock === 'no'}<strong>Out of stock.</strong> {/if}{r.notes}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		{#if info.entry}
			<p class="tiny muted">
				Checked {info.entry.checked}.{info.entry.notes ? ` ${info.entry.notes}` : ''} Per hour of work is the delivered cost divided by the hours the item costs:
				what Hack Club pays for each hour someone there works for it.
			</p>
		{/if}
	{/if}
</div>

<style>
	.prices { display: flex; flex-direction: column; gap: 8px; padding: 4px 0 8px; }
	.prices p { margin: 0; }
	table { font-size: 18px; background: var(--row-bg); border-radius: 10px; overflow: hidden; }
	th { font-size: 16px; padding: 6px 12px; }
	td { padding: 6px 12px; vertical-align: top; }
	.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
	.notes { min-width: 16em; max-width: 34em; }
	.tiny { font-size: 15px; }
	code { font-size: 0.85em; }
</style>
