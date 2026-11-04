<script>
	import ProjectCard from '#lib/components/ProjectCard.svelte';
	import HourBar from '#lib/components/HourBar.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, STATUS } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	let filter = $state('all');
	const shown = $derived(filter === 'all' ? d.projects : d.projects.filter((p) => p.status === filter));
	const count = (s) => d.projects.filter((p) => p.status === s).length;
</script>

<div class="page-head">
	<div class="stack-sm">
		<h1>My projects</h1>
		<p class="muted">{d.projects.length} projects · {d.hours.approved}h approved</p>
	</div>
	<a class="btn primary" href="/projects/new">New project</a>
</div>

{#if d.projects.length}
	<div class="card" style="margin-bottom: 24px"><HourBar hours={d.hours} compact /></div>
	<div class="tabs">
		<button class:active={filter === 'all'} onclick={() => (filter = 'all')}>All ({d.projects.length})</button>
		{#each Object.entries(STATUS) as [key, s]}
			{#if count(key)}
				<button class:active={filter === key} onclick={() => (filter = key)}>{s.label} ({count(key)})</button>
			{/if}
		{/each}
	</div>
	<div class="grid">
		{#each shown as project}
			<ProjectCard {project} />
		{/each}
	</div>
{:else}
	<div class="card dashed" style="align-items: center; text-align: center; padding: 48px">
		<h2>No projects yet</h2>
		<p class="muted">Create a project, link it to Hackatime, and start building. Your hours count once it's shipped and approved.</p>
		<div class="row"><a class="btn primary" href="/projects/new">Create a project</a><a class="btn ghost" href="/guides">Get ideas</a></div>
	</div>
{/if}
