<script>
	import '../app.css';
	import { view, saveView } from '#lib/state.svelte.js';
	import { SCENARIOS } from '#lib/data.js';

	let { children } = $props();
</script>

<!-- Blockout-only controls. Delete this bar when the real platform is built. -->
<div class="blockout-bar">
	<strong>Blockout</strong>
	<a href="/map">Page map</a>
	<label>
		Preview as
		<select bind:value={view.scenario} onchange={saveView}>
			{#each Object.entries(SCENARIOS) as [key, label]}
				<option value={key}>{label}</option>
			{/each}
		</select>
	</label>
	<label>
		<input type="checkbox" bind:checked={view.notes} onchange={saveView} />
		Design notes
	</label>
</div>

{@render children()}
