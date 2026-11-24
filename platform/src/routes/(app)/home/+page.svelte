<script>
	import { onMount } from 'svelte';
	import { get, peek } from '#lib/api.js';
	import Station from '#lib/Station.svelte';
	import HomeCar from '#lib/cars/HomeCar.svelte';
	import BoxTakeover from '#lib/BoxTakeover.svelte';
	import { hours, projectStatus } from '#lib/format.js';

	// Everything starts from what the last visit showed (null the first time),
	// and onMount asks again.
	const ticketIn = (items) => items?.find((i) => i.kind === 'ticket') ?? null;
	let projects = $state(peek('/projects') ?? null);
	// project id → GET /projects/{id}, for unshipped time
	let details = $state(Object.fromEntries((peek('/projects') ?? []).map((p) => [p.id, peek(`/projects/${p.id}`)]).filter(([, d]) => d)));
	let boxes = $state(peek('/boxes') ?? null);
	let specialStamps = $state(peek('/special-stamps') ?? []);
	let ticket = $state(ticketIn(peek('/shop/items')));
	let hackatimeError = $state('');
	let error = $state('');

	onMount(async () => {
		try {
			[projects, boxes, specialStamps] = await Promise.all([get('/projects'), get('/boxes'), get('/special-stamps')]);
		} catch (e) {
			error = e.message;
			return;
		}
		// The bar's goal is what their ticket costs now, which goes up in batches.
		get('/shop/items')
			.then((items) => (ticket = ticketIn(items)))
			.catch(() => {});
		// Check Hackatime early: nothing else works until it can be read.
		get('/hackatime/projects').catch((e) => (hackatimeError = e.message));
		// Unshipped time needs each project's live Hackatime numbers.
		for (const p of projects) {
			if (p.hackatime_projects.length) get(`/projects/${p.id}`).then((d) => (details[p.id] = d)).catch(() => {});
		}
	});

	// Once the takeover opens something, so it and the box count catch up.
	function reloadGifts() {
		Promise.all([get('/boxes'), get('/special-stamps')])
			.then(([b, s]) => ([boxes, specialStamps] = [b, s]))
			.catch(() => {});
	}
</script>

{#if projects && boxes}<BoxTakeover {boxes} stamps={specialStamps} {projects} onopened={reloadGifts} />{/if}

<div class="stack-lg">
	{#if error}<p class="callout warn">{error}</p>{/if}

	<!-- The dashboard is the front car of the train in the layout. -->
	<Station name="home">
		<HomeCar live {projects} {details} {boxes} {ticket} />
	</Station>

	{#if hackatimeError}
		<section class="callout warn" role="alert">
			<strong>We can't read your Hackatime yet</strong>
			<span class="small">{hackatimeError}</span>
			<span class="small">Fix it on <a href="https://auth.hackclub.com" target="_blank" rel="noreferrer">Hack Club Auth</a> or <a href="https://hackatime.hackclub.com" target="_blank" rel="noreferrer">Hackatime</a>. Your hours can't count until this works.</span>
		</section>
	{/if}

	<section class="stack" data-press>
		<div class="row between">
			<h2>Your Projects</h2>
			<a class="btn primary" href="/projects/new">New Project</a>
		</div>
		{#if projects === null}
			<!-- Two rows' worth of space while the list loads, so what's under it
			     doesn't jump when it arrives. -->
			<div class="project-row ghost" role="status"><span class="visually-hidden">Loading your projects…</span></div>
			<div class="project-row ghost" aria-hidden="true"></div>
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
						<span class="meta">{hours(p.approved_seconds)} {p.resubmit_blocked ? 'forfeited' : 'approved'}{#if details[p.id]?.unsubmitted_seconds} · {hours(details[p.id].unsubmitted_seconds)} not shipped{/if}</span>
					</span>
					<span class="badge {s.tone}">{s.label}</span>
				</a>
			{/each}
		{/if}
	</section>
</div>

<style>
	/* A project row's size with nothing in it yet. Still, not shimmering. */
	.ghost { min-height: 110px; opacity: 0.45; }
	.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
	@media (max-width: 820px) {
		.ghost { min-height: 102px; }
	}
</style>
