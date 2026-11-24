<script>
	// The dashboard car, the front of the train: "Hi Max!", tickets and boxes,
	// and the hours bar with its three tiles (the Figma home frame's hero).
	//
	// Home hands it to the train with its live numbers; on Shop and Boxes the
	// train draws the same car from the numbers last loaded (HomePreview). It
	// is one component so the car looks the same either way: it doesn't change
	// shape as the train starts to move.
	import { session } from '../session.svelte.js';
	import StampIcon from '../StampIcon.svelte';
	import { approvedSeconds, inReview, GOAL_HOURS } from '../format.js';

	// null (or nothing for details) means not loaded yet. live: Home is open,
	// so this is the page's heading.
	let { projects = null, details = {}, boxes = null, ticket = null, live = false } = $props();
	let innerWidth = $state(1440);

	const first = $derived(session.me ? session.me.name.replace(/\s*\(dev\)$/, '').trim().split(/\s+/)[0] : '');
	const unopened = $derived(boxes?.filter((b) => !b.opened_at).length);
	// What a number shows before it's known: a dash, not a zero that looks real.
	const DASH = '–';

	// The three hour buckets, in hours, in the bar's order.
	const approvedH = $derived(projects ? approvedSeconds(projects) / 3600 : 0);
	const reviewH = $derived(
		projects ? projects.reduce((t, p) => t + (inReview(p.submissions[0]?.status) ? (p.submissions[0].approved_seconds ?? p.submissions[0].new_seconds) : 0), 0) / 3600 : 0
	);
	// Time that counted toward two projects at once is unshipped on both, so
	// the sum stops at what they have unshipped overall.
	const unshippedH = $derived.by(() => {
		const ds = Object.values(details).filter(Boolean);
		const sum = ds.reduce((t, d) => t + (d.unsubmitted_seconds ?? 0), 0);
		return Math.min(sum, ...ds.map((d) => d.owner_unsubmitted_seconds ?? Infinity)) / 3600;
	});

	// Known once every project with Hackatime linked has its details: until
	// then the sum is short, and a short number looks as real as a right one.
	const unshippedKnown = $derived(!!projects && projects.every((p) => !p.hackatime_projects?.length || details[p.id]));

	const goal = $derived(ticket && session.me ? ticket.price / session.me.stamps_per_hour : GOAL_HOURS);
	const pct = (h) => Math.min((h / goal) * 100, 100);
	const fmt = (h) => (Number.isInteger(h) ? h : h.toFixed(1)) + 'H';
	const ticks = $derived(
		[approvedH, approvedH + reviewH, approvedH + reviewH + unshippedH]
			.filter((h, i, a) => h > 0 && h < goal - 4 && a.indexOf(h) === i)
			.map((h) => ({ h, left: pct(h) }))
			// Drop a tick that would print on top of the one before it.
			.filter((t, i, a) => i === 0 || t.left - a[i - 1].left >= (innerWidth < 820 ? 16 : 7))
	);
</script>

<svelte:window bind:innerWidth />

<div class="home-hero">
	<div class="stack">
		<svelte:element this={live ? 'h1' : 'h2'} class="hi">Hi {first}!</svelte:element>
		<div class="row">
			<span class="stat">Tickets: <b>{session.me?.stamps}<StampIcon /></b></span>
			<a class="stat" href="/boxes" style="text-decoration: none">Boxes: <b>{unopened ?? DASH}</b></a>
		</div>
	</div>
	<div class="stack-sm hours">
		<div class="hours-ticks" aria-hidden="true">
			{#each ticks as t}<span style="left: {t.left}%">{fmt(t.h)}</span>{/each}
			<span class="goal">{fmt(goal)}</span>
		</div>
		<div class="hours-bar" role="img" aria-label="{fmt(approvedH)} approved, {fmt(reviewH)} in review, {fmt(unshippedH)} not shipped, of {fmt(goal)}">
			<span class="unshipped" style="width: {pct(approvedH + reviewH + unshippedH)}%"></span>
			<span class="review" style="width: {pct(approvedH + reviewH)}%"></span>
			<span class="approved" style="width: {pct(approvedH)}%"></span>
		</div>
		<div class="tiles">
			<div class="tile"><span class="label"><i class="key approved"></i>Approved</span><span class="value">{projects ? fmt(approvedH) : DASH}</span></div>
			<div class="tile"><span class="label"><i class="key review"></i>In review</span><span class="value">{projects ? fmt(reviewH) : DASH}</span></div>
			<div class="tile"><span class="label"><i class="key unshipped"></i>Not shipped</span><span class="value">{unshippedKnown ? fmt(unshippedH) : DASH}</span></div>
		</div>
	</div>
</div>

<style>
	/* The "Hi Max!" car from the Figma home frame; RailTrain draws the car. */
	.home-hero {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 24px 40px;
		align-items: center;
		padding: 22px 32px 26px;
	}
	.hi { font-size: clamp(56px, 5.6vw, 96px); }
	.hours { gap: 8px; }
	.home-hero :global(.hours-ticks) { font-size: 24px; height: 30px; }
	/* Bottom-aligned, so a label that wraps doesn't push its box out of line. */
	.tiles { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; align-items: end; }
	.tile .label { display: inline-flex; align-items: center; gap: 8px; white-space: nowrap; }

	/* The tiles carry a key for the bar's segments (colours in app.css), which
	   differ by pattern as well as shade. */
	.key { display: inline-block; width: 18px; height: 18px; border-radius: 4px; flex: none; }
	.key.approved { background: var(--light); }
	.key.review { background: var(--review-stripes); }
	.key.unshipped { background: var(--navy-edge); }

	@media (max-width: 820px) {
		.home-hero { grid-template-columns: 1fr; padding-inline: 18px; }
		.hi { font-size: 56px; }
		.tiles { gap: 8px; }
		/* Small enough that "Not shipped" stays on one line, level with the others. */
		.tile .label { font-size: 19px; gap: 5px; }
		.key { width: 14px; height: 14px; }
	}
</style>
