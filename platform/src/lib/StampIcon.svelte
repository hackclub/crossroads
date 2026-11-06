<script module>
	// One stamp per page load, shared by every icon on the page, and never
	// the same one as the last load, so a reload always shows a new stamp.
	import olive from './assets/stamps/olive.webp';
	import vanitas from './assets/stamps/vanitas.webp';
	import monet from './assets/stamps/monet.webp';

	const stamps = [olive, vanitas, monet];
	let last = -1;
	try {
		last = Number(sessionStorage.getItem('stamp-icon') ?? -1);
	} catch {
		// No storage (private mode etc.): just pick at random.
	}
	let pick = Math.floor(Math.random() * (stamps.length - 1));
	if (pick >= last && last >= 0) pick += 1; // skip the last one shown
	try {
		sessionStorage.setItem('stamp-icon', String(pick));
	} catch {
		// Fine without it.
	}
	const src = stamps[pick];
</script>

<script>
	// A little postage stamp shown after a stamp count. Pass `label` when the
	// icon stands in for the word "stamps"; leave it out when the word is
	// already there, so screen readers don't say it twice.
	let { label = '', size = '1.05em' } = $props();
</script>

<img class="stamp-icon" {src} alt={label} style="height: {size}" />

<style>
	.stamp-icon {
		display: inline-block;
		width: auto;
		margin-left: 0.25em;
		vertical-align: -0.18em;
		rotate: 6deg;
		filter: drop-shadow(0 1px 1px rgb(0 0 0 / 0.5));
	}
</style>
