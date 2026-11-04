<script>
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { get } from '#lib/api.js';
	import ProjectForm from '#lib/ProjectForm.svelte';

	let project = $state(null);
	let error = $state('');
	onMount(async () => {
		try {
			project = await get(`/projects/${page.params.id}`);
		} catch (e) {
			error = e.message;
		}
	});
</script>

<div class="narrow">
	{#if error}
		<p class="callout warn">{error}</p>
	{:else if project}
		<div class="crumbs"><a href="/projects/{project.id}">← {project.name}</a></div>
		<h1 class="page-title">Project Info</h1>
		<ProjectForm {project} />
	{:else}
		<p class="muted">Loading…</p>
	{/if}
</div>
