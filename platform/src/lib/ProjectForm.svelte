<script>
	// Create or edit a project. The details are a postcard: the screenshot is
	// the picture on the left, and the name, description and links are written
	// on ruled lines on the right, under a postmark. Hackatime and Save follow.
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
		<section class="postcard" aria-label="Project details">
			<!-- The picture side: the screenshot. -->
			<div class="picture">
				<ImageDrop bind:value={screenshot_url} disabled={locked} />
				<label class="line small-line">
					<span>or link to a screenshot</span>
					<input type="url" bind:value={screenshot_url} placeholder="https://" />
				</label>
			</div>

			<!-- The writing side: name, message and links on ruled lines. -->
			<div class="writing">
				<div class="franking" aria-hidden="true">
					<span class="postmark">Crossroads<br />Paris</span>
				</div>
				<label class="line name-line">
					<span>Project</span>
					<input required maxlength="100" bind:value={name} placeholder="My cool project" />
					<em>{name.length}/100</em>
				</label>
				<label class="message">
					<span>Description</span>
					<textarea maxlength="5000" bind:value={description} placeholder="What it does, who it's for, and what you built it with."></textarea>
				</label>
				<label class="line">
					<span>Demo</span>
					<input type="url" bind:value={demo_url} placeholder="https://" />
				</label>
				<label class="line">
					<span>Repo</span>
					<input type="url" bind:value={repo_url} placeholder="https://" />
				</label>
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
	.f-label { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 28px; }

	/* ---- The postcard, in the ship postcards' card stock ---- */
	.postcard {
		--paper: #efe9dc;
		--ink: #2b2522;
		--ink-soft: #5c524c;
		--rule: rgb(43 37 34 / 0.32);
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		/* The same shape as the project page's postcard. */
		aspect-ratio: var(--postcard-ratio);
		gap: 28px 32px;
		padding: 28px 32px 32px;
		color: var(--ink);
		text-shadow: none;
		background: linear-gradient(rgb(239 233 220 / 0.94), rgb(239 233 220 / 0.94)), var(--leak);
		border-radius: 6px;
		box-shadow: 0 2px 4px rgb(0 0 0 / 0.3), 0 18px 40px rgb(0 0 0 / 0.5);
	}
	/* The screenshot, printed on the card with a white border. */
	.picture { display: flex; flex-direction: column; gap: 12px; }
	/* The picture fills the side, down to the link line. */
	.picture :global(.drop-wrap) { flex: 1; display: flex; flex-direction: column; }
	.picture :global(.screenshot-well) {
		flex: 1;
		min-height: 360px;
		border: 10px solid #fbf8f1;
		box-shadow: 0 1px 3px rgb(0 0 0 / 0.25);
	}
	.writing {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 18px;
		padding-left: 32px;
		border-left: 1px solid var(--rule);
	}
	/* No stamp while writing the card; just the postmark in its corner. */
	.franking { display: flex; align-items: center; align-self: flex-end; height: 120px; margin-bottom: -6px; }
	.postmark {
		display: grid;
		place-content: center;
		width: 96px;
		height: 96px;
		font-size: 15px;
		line-height: 1.15;
		letter-spacing: 0.08em;
		text-align: center;
		text-transform: uppercase;
		color: #2f4566;
		border: 3px solid #2f4566;
		border-radius: 50%;
		rotate: -14deg;
		opacity: 0.8;
	}

	/* A field written on a ruled line, with its name at the start of it. */
	/* Names share one width, so what's written on every line starts together. */
	.line { display: grid; grid-template-columns: 96px minmax(0, 1fr) auto; align-items: baseline; column-gap: 12px; border-bottom: 1px solid var(--rule); }
	.line span, .message span { font-size: 22px; color: var(--ink-soft); }
	.line em { font-style: normal; font-size: 17px; color: var(--ink-soft); }
	.postcard input, .postcard textarea {
		padding: 8px 0 6px;
		font-size: 26px;
		color: var(--ink);
		caret-color: var(--ink);
		background: transparent;
		border: 0;
		border-radius: 0;
		box-shadow: none;
	}
	.postcard input::placeholder, .postcard textarea::placeholder { color: var(--ink-soft); font-style: italic; }
	.postcard :focus-visible { outline: 2px solid var(--ink); outline-offset: 3px; box-shadow: none; }
	.name-line input { font-size: 34px; }
	.small-line { grid-template-columns: auto minmax(0, 1fr); }
	.small-line span { font-size: 19px; }
	.small-line input { font-size: 21px; }
	/* The message, written across ruled lines 40px apart. */
	.message { display: flex; flex-direction: column; gap: 4px; }
	.message textarea {
		min-height: 200px;
		padding: 0;
		font-size: 24px;
		line-height: 40px;
		background: repeating-linear-gradient(transparent 0 39px, var(--rule) 39px 40px);
		background-attachment: local;
	}

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
		.postcard { grid-template-columns: 1fr; aspect-ratio: auto; padding: 20px 18px 24px; }
		.writing { padding: 20px 0 0; border-left: 0; border-top: 1px solid var(--rule); }
		.picture :global(.screenshot-well) { min-height: 220px; }
		.card-grey { padding-inline: 18px; }
		.ht li { flex-wrap: wrap; row-gap: 4px; }
		.ht a { order: 3; flex-basis: 100%; padding-left: 36px; }
	}
</style>
