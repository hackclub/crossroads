<script>
	// Shared by "New project" and "Edit project".
	import Note from './Note.svelte';
	import Img from './Img.svelte';
	import { PROJECT_TYPES, hackatimeProjects } from '#lib/data.js';

	let { project = null } = $props();

	const editing = !!project;
	let title = $state(project?.title ?? '');
	let description = $state(project?.description ?? '');
	let codeUrl = $state(project?.codeUrl ?? '');
	let readmeUrl = $state(project?.readmeUrl ?? '');
	let usedAi = $state(!!project?.aiUse);
	let otherProgram = $state(false);
	let linked = $state(hackatimeProjects.filter((h) => h.linkedTo === project?.id).map((h) => h.name));

	const linkedHours = $derived(
		hackatimeProjects.filter((h) => linked.includes(h.name)).reduce((t, h) => t + h.hours, 0)
	);

	// Guess the README link from a GitHub repo link, like Beest does.
	function fillReadme() {
		if (!readmeUrl && /github\.com\/[^/]+\/[^/]+/.test(codeUrl))
			readmeUrl = codeUrl.replace(/\/$/, '') + '/blob/main/README.md';
	}
</script>

<form class="stack" onsubmit={(e) => e.preventDefault()}>
	<fieldset>
		<legend>Basics</legend>
		<label class="field">
			<span>Project name <em>{title.length}/50</em></span>
			<input type="text" maxlength="50" bind:value={title} placeholder="Metro Map Maker" />
		</label>
		<label class="field">
			<span>Description <em>{description.length}/300</em></span>
			<textarea maxlength="300" bind:value={description} placeholder="What it does, who it's for, and what you built it with."></textarea>
			<small>At least 30 characters before you can ship.</small>
		</label>
		<label class="field">
			<span>Type</span>
			<select value={project?.type ?? ''}>
				<option value="" disabled>Pick one</option>
				{#each PROJECT_TYPES as t}<option>{t}</option>{/each}
			</select>
			<small>Reviewers test each type differently. See the shipping guide for yours.</small>
		</label>
	</fieldset>

	<fieldset>
		<legend>Screenshot or banner</legend>
		{#if project?.hasBanner}
			<div class="row" style="align-items: flex-start">
				<div style="width: 200px"><Img label="Current banner" /></div>
				<button class="btn small ghost" type="button">Replace</button>
				<button class="btn small ghost" type="button">Remove</button>
			</div>
		{:else}
			<div class="dropzone">Drop an image here or <button class="link-btn" type="button">browse</button><br /><span class="small">PNG, JPG or GIF, up to 5 MB</span></div>
		{/if}
	</fieldset>

	<fieldset>
		<legend>Links <span class="muted small">(needed to ship)</span></legend>
		<label class="field">
			<span>Code repository</span>
			<input type="url" bind:value={codeUrl} onblur={fillReadme} placeholder="https://github.com/you/project" />
			<small>The root of a public repo.</small>
		</label>
		<label class="field">
			<span>Demo</span>
			<input type="url" value={project?.demoUrl ?? ''} placeholder="https://…" />
			<small>Somewhere anyone can try it: a website, a release download, TestFlight, an APK, or a video for hardware.</small>
		</label>
		<label class="field">
			<span>README</span>
			<input type="url" bind:value={readmeUrl} placeholder="https://github.com/you/project/blob/main/README.md" />
			<small>Filled in from your repo link. It should explain what the project is and how to run it.</small>
		</label>
		<Note from="Horizons">Check each link live and show "Link unreachable" under the field when it fails.</Note>
	</fieldset>

	<fieldset>
		<legend>Hackatime projects</legend>
		<p class="small muted">Pick which Hackatime projects count toward this one. Each can only be linked to one project.</p>
		<ul class="list">
			{#each hackatimeProjects as h}
				{@const taken = h.linkedTo && h.linkedTo !== project?.id}
				<li>
					<label class="check" style="flex: 1">
						<input type="checkbox" value={h.name} bind:group={linked} disabled={taken} />
						<span class:muted={taken}>{h.name}{#if taken} <span class="small">(linked to another project)</span>{/if}</span>
					</label>
					<span class="small muted">{h.hours}h</span>
				</li>
			{/each}
		</ul>
		<div class="row between small">
			<strong>{linkedHours.toFixed(1)}h selected</strong>
			<button class="btn small ghost" type="button">Refresh from Hackatime</button>
		</div>
		{#if editing && project.approvedHours}
			<p class="callout warn small">Unlinking a project with approved hours doesn't remove those hours, but the next ship is measured from what's left.</p>
		{/if}
	</fieldset>

	<fieldset>
		<legend>Honesty</legend>
		<label class="check"><input type="checkbox" bind:checked={usedAi} /> I used AI tools on this project</label>
		{#if usedAi}
			<label class="field">
				<span>How did you use AI? <em>200 max</em></span>
				<textarea maxlength="200" value={project?.aiUse ?? ''} placeholder="e.g. Copilot autocomplete, asked ChatGPT to explain an error"></textarea>
			</label>
		{/if}
		<label class="check"><input type="checkbox" bind:checked={otherProgram} /> I've submitted this project to another Hack Club program</label>
		{#if otherProgram}
			<label class="field"><span>Which one?</span><input type="text" placeholder="e.g. Summer of Making" /></label>
			<small class="muted">Only hours tracked after that program's submission can count here.</small>
		{/if}
		<Note from="Beest / Horizons / Flavortown">All three ask for an AI declaration and double-dipping across programs.</Note>
	</fieldset>

	<div class="row">
		<button class="btn primary" type="submit">{editing ? 'Save changes' : 'Create project'}</button>
		<a class="btn ghost" href={editing ? `/projects/${project.id}` : '/projects'}>Cancel</a>
	</div>

	{#if editing}
		<fieldset>
			<legend>Danger zone</legend>
			<div class="row between">
				<span class="small">Delete this project. Approved hours stay on your total.</span>
				<button class="btn small danger" type="button" disabled={project.status === 'in_review'}>Delete project</button>
			</div>
			{#if project.status === 'in_review'}<small class="muted">You can't delete a project while it's in review.</small>{/if}
		</fieldset>
	{/if}
</form>
