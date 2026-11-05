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

{#if error}
	<p class="callout warn">{error}</p>
{:else if project}
	<ProjectForm {project} />
{:else}
	<p class="muted">Loading…</p>
{/if}
