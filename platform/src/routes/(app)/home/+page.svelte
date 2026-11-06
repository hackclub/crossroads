<script>
	import { onMount } from 'svelte';
	import { get } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import Train from '#lib/Train.svelte';
	import StampIcon from '#lib/StampIcon.svelte';
	import { hours, approvedSeconds, projectStatus, GOAL_HOURS } from '#lib/format.js';

	let projects = $state(null);
	let details = $state({}); // project id → GET /projects/{id}, for unshipped time
	let boxes = $state([]);
	let ticket = $state(null);
	let hackatimeError = $state('');
	let error = $state('');
	let innerWidth = $state(1440);

	onMount(async () => {
		try {
			[projects, boxes] = await Promise.all([get('/projects'), get('/boxes')]);
		} catch (e) {
			error = e.message;
			return;
		}
		// The bar's goal is what their ticket costs now, which goes up in batches.
		get('/shop/items')
			.then((items) => (ticket = items.find((i) => i.kind === 'ticket') ?? null))
			.catch(() => {});
		// Check Hackatime early: nothing else works until it can be read.
		get('/hackatime/projects').catch((e) => (hackatimeError = e.message));
		// Unshipped time needs each project's live Hackatime numbers.
		for (const p of projects) {
			if (p.hackatime_projects.length) get(`/projects/${p.id}`).then((d) => (details[p.id] = d)).catch(() => {});
		}
	});

	const first = $derived(session.me.name.replace(/\s*\(dev\)$/, '').split(' ')[0]);
	const stamps = $derived(session.me.stamps);
	const unopened = $derived(boxes.filter((b) => !b.opened_at).length);

	// The three hour buckets, in hours, in the bar's order.
	const approvedH = $derived(projects ? approvedSeconds(projects) / 3600 : 0);
	const reviewH = $derived(
		projects ? projects.reduce((t, p) => t + (p.submissions[0]?.status === 'pending' ? p.submissions[0].new_seconds : 0), 0) / 3600 : 0
	);
	const unshippedH = $derived(Object.values(details).reduce((t, d) => t + (d.unsubmitted_seconds ?? 0), 0) / 3600);

	const goal = $derived(ticket ? ticket.price / session.me.stamps_per_hour : GOAL_HOURS);
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

<div class="stack-lg">
	{#if error}<p class="callout warn">{error}</p>{/if}

	<Train label="Your progress">
		<div class="home-hero">
			<div class="stack">
				<h1>Hi {first}!</h1>
				<div class="row">
					<span class="stat">Stamps: <b>{stamps}<StampIcon /></b></span>
					<a class="stat" href="/boxes" style="text-decoration: none">Boxes: <b>{unopened}</b></a>
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
					<div class="tile"><span class="label"><i class="key approved"></i>Approved</span><span class="value">{fmt(approvedH)}</span></div>
					<div class="tile"><span class="label"><i class="key review"></i>In review</span><span class="value">{fmt(reviewH)}</span></div>
					<div class="tile"><span class="label"><i class="key unshipped"></i>Not shipped</span><span class="value">{fmt(unshippedH)}</span></div>
				</div>
			</div>
		</div>
	</Train>

	{#if hackatimeError}
		<section class="callout warn" role="alert">
			<strong>We can't read your Hackatime yet</strong>
			<span class="small">{hackatimeError}</span>
			<span class="small">Fix it on <a href="https://auth.hackclub.com" target="_blank" rel="noreferrer">Hack Club Auth</a> or <a href="https://hackatime.hackclub.com" target="_blank" rel="noreferrer">Hackatime</a>. Your hours can't count until this works.</span>
		</section>
	{/if}

	<section class="stack">
		<div class="row between">
			<h2>Your Projects</h2>
			<a class="btn primary" href="/projects/new">New Project</a>
		</div>
		{#if projects === null}
			<p class="muted">Loading…</p>
		{:else if projects.length === 0}
			<a class="card dashed" href="/projects/new">
				<span>No projects yet</span>
				<span class="small muted">Create one and link your Hackatime projects to it. Ship it when it works to get its hours approved.</span>
			</a>
		{:else}
			{#each projects as p}
				{@const s = projectStatus(p)}
				<a class="project-row" href="/projects/{p.id}">
					<span class="stack-sm">
						<span class="name">{p.name}</span>
						<span class="meta">{hours(p.approved_seconds)} approved{#if details[p.id]?.unsubmitted_seconds} · {hours(details[p.id].unsubmitted_seconds)} not shipped{/if}</span>
					</span>
					<span class="badge {s.tone}">{s.label}</span>
				</a>
			{/each}
		{/if}
	</section>
</div>

<style>
	/* The "Hi Max!" car from the Figma home frame; Train draws the car. */
	.home-hero {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 24px 40px;
		align-items: center;
		padding: 22px 32px 26px;
	}
	.home-hero h1 { font-size: clamp(56px, 5.6vw, 96px); }
	.hours { gap: 8px; }
	.home-hero :global(.hours-ticks) { font-size: 24px; height: 30px; }
	/* Bottom-aligned, so a label that wraps doesn't push its box out of line. */
	.tiles { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; align-items: end; }
	.tile .label { display: inline-flex; align-items: center; gap: 8px; white-space: nowrap; }

	/* Segments differ by pattern, not only shade, and the tiles carry a key. */
	/* Figma colours, plus a pattern on "in review" so segments differ by more
	   than shade; the tiles carry a matching key. */
	.home-hero :global(.hours-bar .review),
	.key.review { background: repeating-linear-gradient(-45deg, var(--navy-deep) 0 5px, #232b47 5px 10px); }
	.key.unshipped { background: var(--navy); box-shadow: inset 0 0 0 2px #4a5680; }
	.key { display: inline-block; width: 18px; height: 18px; border-radius: 4px; flex: none; }
	.key.approved { background: var(--deepest); box-shadow: inset 0 0 0 2px #6b6262; }

	@media (max-width: 820px) {
		.home-hero { grid-template-columns: 1fr; padding-inline: 18px; }
		.home-hero h1 { font-size: 56px; }
		.tiles { gap: 8px; }
		.tile .label { white-space: normal; text-align: center; }
	}
</style>
