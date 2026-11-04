<script>
	import { onMount } from 'svelte';
	import { get } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import { hours, approvedSeconds, projectStatus, GOAL_HOURS } from '#lib/format.js';

	let projects = $state(null);
	let details = $state({}); // project id → GET /projects/{id}, for unsubmitted time
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
		// Unsubmitted time needs each project's live Hackatime numbers.
		for (const p of projects) {
			if (p.hackatime_projects.length) get(`/projects/${p.id}`).then((d) => (details[p.id] = d)).catch(() => {});
		}
	});

	const first = $derived(session.me.name.replace(/\s*\(dev\)$/, '').split(' ')[0]);
	const ticket = $derived(items.find((i) => i.kind === 'ticket'));
	const ticketOrder = $derived(orders.find((o) => o.item.kind === 'ticket' && o.status !== 'cancelled'));
	const unopened = $derived(boxes.filter((b) => !b.opened_at).length);

	// The three hour buckets, in hours.
	const approvedH = $derived(projects ? approvedSeconds(projects) / 3600 : 0);
	const pendingH = $derived(
		projects ? projects.reduce((t, p) => t + (p.submissions[0]?.status === 'pending' ? p.submissions[0].new_seconds : 0), 0) / 3600 : 0
	);
	const unsubmittedH = $derived(Object.values(details).reduce((t, d) => t + (d.unsubmitted_seconds ?? 0), 0) / 3600);

	const pct = (h) => Math.min((h / GOAL_HOURS) * 100, 100);
	const fmt = (h) => (Number.isInteger(h) ? h : h.toFixed(1)) + 'H';
	// Ticks at each bucket's end, like 19H / 26H / 48H in the Figma.
	const ticks = $derived(
		[approvedH, approvedH + pendingH, approvedH + pendingH + unsubmittedH]
			.filter((h, i, a) => h > 0 && h < GOAL_HOURS && a.indexOf(h) === i)
			.map((h) => ({ h, left: pct(h) }))
	);
</script>

<div class="stack-lg">
	{#if error}<p class="callout warn">{error}</p>{/if}

	<section class="home-hero">
		<div class="stack">
			<h1>Hi {first}!</h1>
			<div class="row">
				<span class="stat">Stamps: <b>{session.me.stamps}</b></span>
				<a class="stat" href="/boxes" style="text-decoration: none">Boxes: <b>{unopened}</b></a>
			</div>
		</div>
		<div class="stack-sm hours">
			<div class="hours-ticks" aria-hidden="true">
				{#each ticks as t}<span style="left: {t.left}%">{fmt(t.h)}</span>{/each}
				<span class="goal">{GOAL_HOURS}H</span>
			</div>
			<div class="hours-bar" role="img" aria-label="{fmt(approvedH)} approved, {fmt(pendingH)} pending, {fmt(unsubmittedH)} not submitted, of {GOAL_HOURS}H">
				<span class="unsubmitted" style="width: {pct(approvedH + pendingH + unsubmittedH)}%"></span>
				<span class="pending" style="width: {pct(approvedH + pendingH)}%"></span>
				<span class="approved" style="width: {pct(approvedH)}%"></span>
			</div>
			<div class="tiles">
				<div class="tile"><span class="label">Not Submitted</span><span class="value">{fmt(unsubmittedH)}</span></div>
				<div class="tile"><span class="label">Pending</span><span class="value">{fmt(pendingH)}</span></div>
				<div class="tile"><span class="label">Approved</span><span class="value">{fmt(approvedH)}</span></div>
			</div>
		</div>
	</section>

	{#if hackatimeError}
		<section class="callout warn">
			<strong>We can't read your Hackatime yet</strong>
			<span class="small">{hackatimeError}</span>
			<span class="small">Fix it on <a href="https://auth.hackclub.com" target="_blank" rel="noreferrer">Hack Club Auth</a> or <a href="https://hackatime.hackclub.com" target="_blank" rel="noreferrer">Hackatime</a>. Your hours can't count until this works.</span>
		</section>
	{/if}

	{#if ticketOrder}
		<p class="callout good">You have your ticket! See you in Paris. <span class="small">Order #{ticketOrder.id} · {ticketOrder.status}</span></p>
	{:else if ticket}
		<div class="callout {session.me.stamps >= ticket.price ? 'good' : ''}">
			<div class="row between">
				<span>{session.me.stamps} / {ticket.price} stamps for the trip ticket</span>
				{#if session.me.stamps >= ticket.price}<a class="btn primary small" href="/shop">Buy your ticket</a>{:else}<span class="small muted">{ticket.price - session.me.stamps} to go</span>{/if}
			</div>
			<span class="small muted">Buying the ticket is how you qualify. Spending stamps on other things pushes it further away.</span>
		</div>
	{/if}

	<section class="stack">
		<div class="row between">
			<h2>Your Projects</h2>
			<a class="btn primary" href="/projects/new">New Project</a>
		</div>
		{#if projects === null}
			<p class="muted">Loading…</p>
		{:else if projects.length === 0}
			<div class="card dashed">
				<span>No projects yet</span>
				<span class="small muted">Create one and link your Hackatime projects to it. Ship it when it works to get its hours approved.</span>
			</div>
		{:else}
			{#each projects as p}
				{@const s = projectStatus(p)}
				<a class="project-row" href="/projects/{p.id}">
					<span class="stack-sm">
						<span class="name">{p.name}</span>
						<span class="meta">Hours: {hours(p.approved_seconds).replace('h', '')} approved{#if details[p.id]?.unsubmitted_seconds} · {hours(details[p.id].unsubmitted_seconds)} not shipped{/if}</span>
					</span>
					<span class="badge {s.tone}">{s.label}</span>
				</a>
			{/each}
		{/if}
	</section>
</div>

<style>
	/* The "Hi Max!" card from the Figma home frame. */
	.home-hero {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 24px 40px;
		align-items: center;
		padding: 22px 32px 26px;
		background: var(--head);
		border-radius: var(--radius);
	}
	.hours { gap: 8px; }
	.tiles { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; }
	.home-hero h1 { font-size: clamp(56px, 5.6vw, 96px); }
	@media (max-width: 820px) {
		.home-hero { grid-template-columns: 1fr; padding-inline: 18px; }
	}
</style>
