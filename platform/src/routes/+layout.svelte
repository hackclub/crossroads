<script>
	import '../app.css';
	import favicon from '#lib/assets/favicon.png'; // the crossing sign from the top bar
	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" type="image/png" href={favicon} />
</svelte:head>

{@render children()}
