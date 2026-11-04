<script>
	// A design note for the blockout: why something is here, or where the idea came from.
	// Hidden with the "Notes" toggle in the blockout bar.
	import { view } from '#lib/state.svelte.js';
	let { from = '', children } = $props();
</script>

{#if view.notes}
	<aside class="note">
		{#if from}<strong>From {from}:</strong>{/if}
		{@render children()}
	</aside>
{/if}
