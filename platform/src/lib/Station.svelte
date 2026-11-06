<script>
	// Hands this page's car content to the long train in the layout
	// (RailTrain.svelte). Renders nothing where it's placed.
	import { rail } from './rail.svelte.js';

	let { name, children } = $props();

	$effect(() => {
		rail[name] = children;
		return () => {
			if (rail[name] === children) rail[name] = null;
		};
	});
</script>
