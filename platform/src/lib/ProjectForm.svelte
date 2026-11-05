<script>
	// Create or edit a project. Laid out like the home page: a centered title,
	// grey cards holding the fields, and a "Next"-style bar with Save.
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { api, get } from './api.js';
	import { hours } from './format.js';
	import ImageDrop from './ImageDrop.svelte';

	let { project: projectProp = null } = $props();
	// The form starts from the project once; edits live in the fields below.
	const project = untrack(() => projectProp);
	const editing = !!project;

	let name = $state(project?.name ?? '');
	let description = $state(project?.description ?? '');
	let repo_url = $state(project?.repo_url ?? '');
	let demo_url = $state(project?.demo_url ?? '');
	let screenshot_url = $state(project?.screenshot_url ?? '');
	let linked = $state([...(project?.hackatime_projects ?? [])]);
	let hackatime = $state(null);
	let hackatimeError = $state('');
	let error = $state('');
	let saving = $state(false);

	const locked = project?.submissions?.[0]?.status === 'pending';
	const cancelHref = editing ? `/projects/${project.id}` : '/home';

	onMount(async () => {
		try {
			hackatime = await get('/hackatime/projects');
		} catch (e) {
			hackatimeError = e.message;
		}
	});

	async function save(e) {
		e.preventDefault();
		error = '';
		saving = true;
		const body = { name, description, repo_url, demo_url, screenshot_url, hackatime_projects: linked };
		// PATCH treats "" as "clear it"; on create, just leave empty fields out.
		if (!editing) for (const k of ['repo_url', 'demo_url', 'screenshot_url']) if (!body[k]) delete body[k];
		try {
			const saved = editing ? await api('PATCH', `/projects/${project.id}`, body) : await api('POST', '/projects', body);
			goto(`/projects/${saved.id}`);
		} catch (err) {
			error = err.message;
		} finally {
			saving = false;
		}
	}
</script>

<form class="stack-lg" onsubmit={save}>
	<header class="title">
		{#if editing}<a class="crumb" href={cancelHref}>← {project.name}</a>{/if}
		<h1>{editing ? 'Project Info' : 'New Project'}</h1>
	</header>

	{#if locked}
		<p class="callout info">This project is waiting for review, so it can't be changed until that's done.</p>
	{/if}

	<fieldset class="stack-lg" disabled={locked}>
		<section class="card-grey details">
			<div class="stack">
				<label class="f">
					<span class="f-label">Project Name <em>{name.length}/100</em></span>
					<input required maxlength="100" bind:value={name} placeholder="My cool project" />
				</label>
				<label class="f">
					<span class="f-label">Description</span>
					<textarea maxlength="5000" bind:value={description} placeholder="What it does, who it's for, and what you built it with."></textarea>
				</label>
				<div class="halves">
					<label class="f"><span class="f-label">Demo link</span><input type="url" bind:value={demo_url} placeholder="https://" /></label>
					<label class="f"><span class="f-label">Repo link</span><input type="url" bind:value={repo_url} placeholder="https://" /></label>
				</div>
			</div>
			<div class="f">
				<span class="f-label">Screenshot</span>
				<div class="shot">
					<ImageDrop bind:value={screenshot_url} disabled={locked} />
					<label class="link-row">
						<span>or link to one:</span>
						<input type="url" bind:value={screenshot_url} placeholder="https://" />
					</label>
				</div>
			</div>
		</section>

		<section class="card-grey stack">
			<div class="f-label">Hackatime Project(s)</div>
			{#if hackatimeError}
				<p class="callout warn small">{hackatimeError}</p>
			{:else if hackatime === null}
				<p class="note">Loading your Hackatime…</p>
			{:else if hackatime.length === 0}
				<p class="note">No Hackatime time since Nov 3 yet. Code a bit with Hackatime running, then come back.</p>
			{:else}
				<ul class="ht">
					{#each hackatime as h}
						{@const elsewhere = h.project_id && h.project_id !== project?.id}
						<li class:elsewhere>
							<label class="check">
								<input type="checkbox" value={h.name} bind:group={linked} disabled={elsewhere} />
								<span>{h.name}</span>
							</label>
							{#if elsewhere}<a href="/projects/{h.project_id}">counts toward another project</a>{/if}
							<b>{hours(h.seconds)}</b>
						</li>
					{/each}
				</ul>
			{/if}
			<p class="note">Each Hackatime project can only count toward one of your projects.</p>
		</section>
	</fieldset>

	{#if error}<p class="callout warn" role="alert">{error}</p>{/if}

	<section class="savebar">
		<span class="text">Only the name is needed to save. Ship it from the project page once every field is filled.</span>
		<a class="btn ghost" href={cancelHref}>Cancel</a>
		<button class="btn primary" disabled={saving || locked}>{saving ? 'Saving…' : 'Save'}</button>
	</section>
</form>

<style>
	/* Same card language as the home page: #8c8c8c cards, 24px corners,
	   darker rounded wells inside. Text on the grey stays ≥24px (large text). */
	/* Black train cars, as on the home page. */
	.card-grey {
		padding: 22px 32px 26px;
		background: var(--car-bg);
		border-radius: var(--radius);
	}
	.title { display: flex; flex-direction: column; }
	.title h1 { font-size: clamp(56px, 5.6vw, 96px); text-align: center; }
	.crumb { align-self: flex-start; font-size: 24px; color: var(--text); }

	fieldset { gap: 32px; }
	.details {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 24px 40px;
		align-items: start;
	}
	.halves { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
	.f { display: flex; flex-direction: column; gap: 8px; }
	.f-label { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 28px; }
	.f-label em { font-style: normal; font-size: 20px; padding: 0 10px; background: var(--navy); border-radius: 999px; }
	.f input, .f textarea { padding: 12px 18px; border-radius: 10px; }
	.f textarea { min-height: 220px; }

	.shot { overflow: hidden; border-radius: 10px; background: var(--navy); }
	.shot :global(.screenshot-well) { min-height: 300px; }
	.link-row { display: flex; align-items: center; gap: 12px; padding: 10px 14px 10px 18px; }
	.link-row span { white-space: nowrap; font-size: 24px; }
	.link-row input { padding: 6px 12px; font-size: 22px; border-radius: 6px; }

	.ht { display: grid; gap: 10px; margin: 0; padding: 0; list-style: none; }
	.ht li {
		display: flex;
		align-items: center;
		gap: 16px;
		padding: 10px 10px 10px 18px;
		font-size: 26px;
		background: var(--navy);
		border-radius: 10px;
	}
	.ht .check { flex: 1; min-width: 0; }
	.ht li.elsewhere .check span { color: var(--muted); }
	.ht a { font-size: 21px; color: var(--muted); }
	.ht b { font-weight: 400; min-width: 72px; padding: 4px 14px; text-align: center; background: var(--navy-deep); border-radius: 10px; }
	.note { margin: 0; font-size: 24px; }

	/* The home page's "Next" bar, holding Save. */
	.savebar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px 16px;
		padding: 16px 16px 16px 28px;
		background: var(--row-bg);
		border-radius: var(--radius);
	}
	.savebar .text { flex: 1; min-width: 260px; font-size: 24px; }
	.savebar .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); }

	@media (max-width: 820px) {
		.details { grid-template-columns: 1fr; }
		.card-grey { padding-inline: 18px; }
		.halves { grid-template-columns: 1fr; }
		.ht li { flex-wrap: wrap; row-gap: 4px; }
		.ht a { order: 3; flex-basis: 100%; padding-left: 36px; }
	}
</style>
