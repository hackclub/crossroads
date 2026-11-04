<script>
	// Create or edit a project, laid out like the Figma "project edit" frame:
	// Project Name, the screenshot block, Description, Demo / Repo links side
	// by side, Hackatime Project(s), then Save.
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

{#if locked}
	<p class="callout info">This project is waiting for review, so it can't be changed until that's done.</p>
{/if}

<form onsubmit={save}>
	<fieldset class="stack" disabled={locked}>
		<label class="field">
			<span>Project Name <em>{name.length}/100</em></span>
			<input required maxlength="100" bind:value={name} placeholder="My cool project" />
		</label>

		<div class="field">
			<span class="field-head">Screenshot</span>
			<ImageDrop bind:value={screenshot_url} disabled={locked} />
			<label class="link-row">
				<span class="small muted">or link to one:</span>
				<input id="screenshot" type="url" bind:value={screenshot_url} placeholder="https://" />
			</label>
		</div>

		<label class="field">
			<span>Description</span>
			<textarea maxlength="5000" bind:value={description} placeholder="What it does, who it's for, and what you built it with."></textarea>
		</label>

		<div class="grid-halves">
			<label class="field"><span>Demo link</span><input type="url" bind:value={demo_url} placeholder="https://" /></label>
			<label class="field"><span>Repo link</span><input type="url" bind:value={repo_url} placeholder="https://" /></label>
		</div>

		<div class="field">
			<span class="field-head">Hackatime Project(s)</span>
			<div class="field-body">
				{#if hackatimeError}
					<p class="callout warn small">{hackatimeError}</p>
				{:else if hackatime === null}
					<p class="small muted">Loading your Hackatime…</p>
				{:else if hackatime.length === 0}
					<p class="small muted">No Hackatime time since Nov 3 yet. Code a bit with Hackatime running, then come back.</p>
				{:else}
					<ul class="list">
						{#each hackatime as h}
							{@const elsewhere = h.project_id && h.project_id !== project?.id}
							<li>
								<label class="check" style="flex: 1">
									<input type="checkbox" value={h.name} bind:group={linked} disabled={elsewhere} />
									<span class:muted={elsewhere}>{h.name}</span>
								</label>
								{#if elsewhere}<a class="small muted" href="/projects/{h.project_id}">counts toward another project</a>{/if}
								<span class="small muted">{hours(h.seconds)}</span>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
			<small>Each Hackatime project can only count toward one of your projects. Only the name is needed now; shipping needs every field.</small>
		</div>
	</fieldset>

	{#if error}<p class="callout warn" style="margin-top: 16px">{error}</p>{/if}
	<div class="row" style="justify-content: flex-end; margin-top: 20px">
		<a class="btn ghost" href={editing ? `/projects/${project.id}` : '/home'}>Cancel</a>
		<button class="btn" disabled={saving || locked}>{saving ? 'Saving…' : 'Save'}</button>
	</div>
</form>

<style>
	.field-head {
		display: block;
		padding: 10px 24px;
		background: var(--head);
		font-size: 30px;
	}
	.field-body { padding: 8px 24px 12px; background: var(--body); }
	.field-body :global(.list li) { border-color: var(--deep); }
	.link-row { display: flex; align-items: center; gap: 12px; padding: 8px 24px; background: var(--body); }
	.link-row span { white-space: nowrap; }
	.link-row input { padding: 6px 12px; font-size: 22px; background: var(--deep); border-radius: 6px; }
</style>
