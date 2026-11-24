<script>
	// The dashboard car while you're on Shop or Boxes: the same car Home shows
	// (HomeCar), drawn from what was last loaded, so it doesn't change as the
	// train pulls away from Home or back to it.
	import { get, peek } from '../api.js';
	import HomeCar from './HomeCar.svelte';

	const ticketIn = (items) => items?.find((i) => i.kind === 'ticket') ?? null;
	const detailsOf = (projects) => Object.fromEntries((projects ?? []).map((p) => [p.id, peek(`/projects/${p.id}`)]).filter(([, d]) => d));

	let projects = $state(peek('/projects') ?? null);
	let boxes = $state(peek('/boxes') ?? null);
	let ticket = $state(ticketIn(peek('/shop/items')));
	// Unshipped time comes from Hackatime through each project's own page, so
	// this only uses what Home already asked for.
	const details = $derived(detailsOf(projects));

	$effect(() => {
		get('/projects').then((p) => (projects = p)).catch(() => {});
		get('/boxes').then((b) => (boxes = b)).catch(() => {});
		get('/shop/items').then((items) => (ticket = ticketIn(items))).catch(() => {});
	});
</script>

<HomeCar {projects} {details} {boxes} {ticket} />
