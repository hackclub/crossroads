<script>
	// This week, last week, this month, last month, all time, or any two days
	// (Eastern, both included). onchange runs after from and to change.
	import { PRESETS } from './period.js';

	let { from = $bindable(), to = $bindable(), onchange } = $props();
	let preset = $state('week');

	function choose(key) {
		preset = key;
		[from, to] = PRESETS.find(([k]) => k === key)[2]();
		onchange();
	}
	// A start after the end moves the end to it, and the other way round.
	function custom(changed) {
		preset = null;
		if (!from || !to) return;
		if (from > to) {
			if (changed === 'from') to = from;
			else from = to;
		}
		onchange();
	}
</script>

<div class="row filters">
	<div class="presets" role="group" aria-label="Period">
		{#each PRESETS as [key, label] (key)}
			<button class="btn small" class:primary={preset === key} aria-pressed={preset === key} onclick={() => choose(key)}>{label}</button>
		{/each}
	</div>
	<div class="range">
		<label>From <input type="date" bind:value={from} onchange={() => custom('from')} /></label>
		<label>to <input type="date" bind:value={to} onchange={() => custom('to')} /></label>
	</div>
</div>

<style>
	.filters { justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
	.presets { display: flex; flex-wrap: wrap; gap: 6px; }
	.range { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; font-size: 22px; }
	.range label { display: inline-flex; align-items: center; gap: 8px; }
	.range input { width: auto; padding: 4px 10px; font-size: 20px; }
</style>
