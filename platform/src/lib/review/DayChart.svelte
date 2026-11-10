<script>
	// One series per day as bars: Hackatime hours or commits. A single series
	// needs no legend; the title names it. Each bar has a tooltip, and the
	// table view below gives the numbers.
	import { utcDay } from '#lib/format.js';

	let { columns, value, label, summary, format = (v) => v, height = 90 } = $props();
	const max = $derived(Math.max(1, ...columns.map(value)));
</script>

<div class="chart">
	<div class="head small"><span>{label}</span><span class="muted">{summary}</span></div>
	<div class="strip" style="height: {height}px" role="img" aria-label={label}>
		{#each columns as c (c.day)}
			<div class="col" title="{utcDay(c.day)}: {format(value(c))}">
				<div class="bar" style="height: {(value(c) / max) * 100}%"></div>
			</div>
		{/each}
	</div>
	{#if columns.length}
		<div class="axis small muted"><span>{utcDay(columns[0].day)}</span><span>{utcDay(columns[columns.length - 1].day)}</span></div>
	{/if}
</div>

<style>
	.chart { display: flex; flex-direction: column; gap: 4px; }
	.head { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 2px 8px; }
	.strip { display: flex; align-items: flex-end; gap: 2px; padding-bottom: 1px; border-bottom: 1px solid var(--deep); }
	.col { flex: 1; height: 100%; display: flex; align-items: flex-end; min-width: 3px; }
	.col:hover { background: var(--rv-sunk, rgb(255 255 255 / 0.06)); }
	.bar { width: 100%; background: var(--info); border-radius: 4px 4px 0 0; }
	.axis { display: flex; justify-content: space-between; }
</style>
