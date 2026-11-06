<script>
	// The dashboard car while you're on another page: a short version of the
	// Home hero. The full one comes from the Home page (see Station.svelte).
	import { get } from '../api.js';
	import { session } from '../session.svelte.js';
	import StampIcon from '../StampIcon.svelte';

	let unopened = $state(0);
	$effect(() => {
		get('/boxes')
			.then((boxes) => (unopened = boxes.filter((b) => !b.opened_at).length))
			.catch(() => {});
	});
	const first = $derived(session.me ? session.me.name.replace(/\s*\(dev\)$/, '').trim().split(/\s+/)[0] : '');
</script>

<div class="preview">
	<h2>Hi {first}!</h2>
	<div class="row">
		<span class="stat">Stamps: <b>{session.me?.stamps}<StampIcon /></b></span>
		<span class="stat">Boxes: <b>{unopened}</b></span>
	</div>
</div>

<style>
	.preview { display: flex; flex-direction: column; justify-content: center; gap: 14px; height: 100%; padding: 22px 32px 26px; }
	h2 { font-size: clamp(48px, 5vw, 80px); }
</style>
