<script>
	import { onMount } from 'svelte';
	import { get } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import { hours, approvedSeconds, projectStatus, stampsPerHour, GOAL_HOURS } from '#lib/format.js';

	let projects = $state(null);
	let details = $state({}); // project id → GET /projects/{id}, for unshipped time
	let items = $state([]);
	let orders = $state([]);
	let boxes = $state([]);
	let hackatimeError = $state('');
	let error = $state('');

	onMount(async () => {
		try {
			[projects, items, orders, boxes] = await Promise.all([get('/projects'), get('/shop/items'), get('/orders'), get('/boxes')]);
		} catch (e) {
			error = e.message;
			return;
		}
		// Check Hackatime early: nothing else works until it can be read.
		get('/hackatime/projects').catch((e) => (hackatimeError = e.message));
		// Unshipped time needs each project's live Hackatime numbers.
		for (const p of projects) {
			if (p.hackatime_projects.length) get(`/projects/${p.id}`).then((d) => (details[p.id] = d)).catch(() => {});
		}
	});

	const first = $derived(session.me.name.replace(/\s*\(dev\)$/, '').split(' ')[0]);
	const stamps = $derived(session.me.stamps);
	const ticket = $derived(items.find((i) => i.kind === 'ticket'));
	const ticketOrder = $derived(orders.find((o) => o.item.kind === 'ticket' && o.status !== 'cancelled'));
	const unopened = $derived(boxes.filter((b) => !b.opened_at).length);

	// The three hour buckets, in hours, in the bar's order.
	const approvedH = $derived(projects ? approvedSeconds(projects) / 3600 : 0);
	const reviewH = $derived(
		projects ? projects.reduce((t, p) => t + (p.submissions[0]?.status === 'pending' ? p.submissions[0].new_seconds : 0), 0) / 3600 : 0
	);
	const unshippedH = $derived(Object.values(details).reduce((t, d) => t + (d.unsubmitted_seconds ?? 0), 0) / 3600);

	const pct = (h) => Math.min((h / GOAL_HOURS) * 100, 100);
	const fmt = (h) => (Number.isInteger(h) ? h : h.toFixed(1)) + 'H';
	const ticks = $derived(
		[approvedH, approvedH + reviewH, approvedH + reviewH + unshippedH]
			.filter((h, i, a) => h > 0 && h < GOAL_HOURS - 4 && a.indexOf(h) === i)
			.map((h) => ({ h, left: pct(h) }))
	);

	const missingFields = (p) =>
		[!p.description && 'a description', !p.repo_url && 'a repo link', !p.demo_url && 'a demo link', !p.screenshot_url && 'a screenshot', !p.hackatime_projects.length && 'a Hackatime project'].filter(Boolean);
	const list = (xs) => (xs.length < 2 ? xs.join('') : xs.slice(0, -1).join(', ') + ' and ' + xs.at(-1));

	// The single most useful thing to do now (Product Principle 1).
	const next = $derived.by(() => {
		if (!projects) return null;
		if (ticketOrder) return { text: "You're going to Paris! We'll message you on Slack with the trip details.", href: '/shop/orders', cta: 'See your ticket' };
		if (ticket && stamps >= ticket.price) return { text: `You have ${stamps} stamps: enough for your ticket.`, href: '/shop', cta: 'Buy your ticket' };
		if (!projects.length) return { text: 'Start a project and link your Hackatime to it. Your hours count once you ship it.', href: '/projects/new', cta: 'Create a project' };
		const ready = projects.find((p) => p.submissions[0]?.status !== 'pending' && (details[p.id]?.unsubmitted_seconds ?? 0) > 0 && !missingFields(p).length);
		if (ready) return { text: `Ship ${hours(details[ready.id].unsubmitted_seconds)} of new time on ${ready.name}.`, href: `/projects/${ready.id}`, cta: 'Ship it' };
		const blocked = projects.find((p) => p.submissions[0]?.status !== 'pending' && missingFields(p).length);
		if (blocked) return { text: `Add ${list(missingFields(blocked))} to ${blocked.name} so you can ship it.`, href: `/projects/${blocked.id}/edit`, cta: 'Edit project' };
		const waiting = projects.find((p) => p.submissions[0]?.status === 'pending');
		if (waiting) return { text: `${waiting.name} is in review. Keep building while you wait.`, href: `/projects/${waiting.id}`, cta: 'See project' };
		return { text: 'Keep building with Hackatime running, then ship your new time.', href: '/projects/new', cta: 'New project' };
	});
</script>

<div class="stack-lg">
	{#if error}<p class="callout warn">{error}</p>{/if}

	<section class="home-hero">
		<div class="stack">
			<h1>Hi {first}!</h1>
			<div class="row">
				<span class="stat">Stamps: <b>{stamps}</b></span>
				<a class="stat" href="/boxes" style="text-decoration: none">Boxes: <b>{unopened}</b></a>
			</div>
		</div>
		<div class="stack-sm hours">
			<div class="hours-ticks" aria-hidden="true">
				{#each ticks as t}<span style="left: {t.left}%">{fmt(t.h)}</span>{/each}
				<span class="goal">{GOAL_HOURS}H</span>
			</div>
			<div class="hours-bar" role="img" aria-label="{fmt(approvedH)} approved, {fmt(reviewH)} in review, {fmt(unshippedH)} not shipped, of {GOAL_HOURS}H">
				<span class="unshipped" style="width: {pct(approvedH + reviewH + unshippedH)}%"></span>
				<span class="review" style="width: {pct(approvedH + reviewH)}%"></span>
				<span class="approved" style="width: {pct(approvedH)}%"></span>
			</div>
			<div class="tiles">
				<div class="tile"><span class="label"><i class="key approved"></i>Approved</span><span class="value">{fmt(approvedH)}</span></div>
				<div class="tile"><span class="label"><i class="key review"></i>In review</span><span class="value">{fmt(reviewH)}</span></div>
				<div class="tile"><span class="label"><i class="key unshipped"></i>Not shipped</span><span class="value">{fmt(unshippedH)}</span></div>
			</div>
			{#if ticket && !ticketOrder}
				<span class="ticket-line">{Math.max(stamps, 0)} / {ticket.price} stamps for your ticket{#if stamps < ticket.price} · about {Math.ceil((ticket.price - stamps) / stampsPerHour(ticket))}h to go{/if}</span>
			{/if}
		</div>
	</section>

	{#if hackatimeError}
		<section class="callout warn" role="alert">
			<strong>We can't read your Hackatime yet</strong>
			<span class="small">{hackatimeError}</span>
			<span class="small">Fix it on <a href="https://auth.hackclub.com" target="_blank" rel="noreferrer">Hack Club Auth</a> or <a href="https://hackatime.hackclub.com" target="_blank" rel="noreferrer">Hackatime</a>. Your hours can't count until this works.</span>
		</section>
	{/if}

	{#if next}
		<section class="next" aria-label="Next step">
			<span class="label">Next</span>
			<span class="text">{next.text}</span>
			<a class="btn primary" href={next.href}>{next.cta}</a>
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
	/* The "Hi Max!" card from the Figma home frame. Everything on it is at
	   least 24px, where white on #8c8c8c (3.36:1) passes as large text. */
	.home-hero {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 24px 40px;
		align-items: center;
		padding: 22px 32px 26px;
		background: var(--head);
		border-radius: var(--radius);
	}
	.home-hero h1 { font-size: clamp(56px, 5.6vw, 96px); }
	.hours { gap: 8px; }
	.home-hero :global(.hours-ticks) { font-size: 24px; height: 30px; }
	.tiles { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; }
	.tile .label { display: inline-flex; align-items: center; gap: 8px; white-space: nowrap; }
	.ticket-line { font-size: 26px; padding-top: 4px; }

	/* Segments differ by pattern, not only shade, and the tiles carry a key. */
	.home-hero :global(.hours-bar .review),
	.key.review { background: repeating-linear-gradient(-45deg, var(--deepest) 0 5px, var(--deep) 5px 10px); }
	.home-hero :global(.hours-bar .unshipped),
	.key.unshipped { background: var(--body); box-shadow: inset 0 0 0 2px var(--deepest); }
	.key { display: inline-block; width: 18px; height: 18px; border-radius: 4px; flex: none; }
	.key.approved { background: var(--deepest); }

	.next {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px 20px;
		padding: 16px 16px 16px 28px;
		background: var(--body);
		border-radius: var(--radius);
	}
	.next .label { font-size: 26px; padding: 2px 14px; background: var(--deep); border-radius: 10px; }
	.next .text { flex: 1; min-width: 260px; font-size: 28px; }

	@media (max-width: 820px) {
		.home-hero { grid-template-columns: 1fr; padding-inline: 18px; }
		.tiles { gap: 8px; }
		.tile .label { white-space: normal; text-align: center; }
	}
</style>
