<script>
	import { page } from '$app/state';
	import ProjectForm from '#lib/components/ProjectForm.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, findProject } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	const project = $derived(findProject(d, page.params.id));
</script>

<div class="main narrow" style="padding: 0">
	{#if project}
		<div class="crumbs"><a href="/projects">My projects</a> / <a href="/projects/{project.id}">{project.title}</a> / Edit</div>
		<div class="page-head"><h1>Edit project</h1></div>
		{#if project.status === 'in_review'}
			<p class="callout info" style="margin-bottom: 16px">This project is in review, so changes won't be seen until the review is done. <button class="link-btn">Withdraw from review</button> to edit and ship again.</p>
		{/if}
		{#key project.id}<ProjectForm {project} />{/key}
	{:else}
		<h1>Project not found</h1>
	{/if}
</div>
