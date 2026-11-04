<script>
	import ProjectCard from '#lib/components/ProjectCard.svelte';
	import Note from '#lib/components/Note.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, PROJECT_TYPES } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	let q = $state('');
	let type = $state('');
	let sort = $state('likes');

	const shipped = $derived(
		d.allProjects
			.filter((p) => p.status === 'approved')
			.filter((p) => !type || p.type === type)
			.filter((p) => !q || (p.title + p.description + p.owner).toLowerCase().includes(q.toLowerCase()))
			.toSorted((a, b) => (sort === 'likes' ? (b.likes ?? 0) - (a.likes ?? 0) : b.approvedHours - a.approvedHours))
	);
</script>

<div class="page-head">
	<div class="stack-sm">
		<h1>Explore</h1>
		<p class="muted">Projects other Crossroads builders have shipped.</p>
	</div>
</div>

<div class="row" style="margin-bottom: 20px">
	<input type="search" placeholder="Search projects or people" bind:value={q} style="max-width: 320px" />
	<select bind:value={type} style="max-width: 180px">
		<option value="">All types</option>
		{#each PROJECT_TYPES as t}<option>{t}</option>{/each}
	</select>
	<select bind:value={sort} style="max-width: 180px">
		<option value="likes">Most liked</option>
		<option value="hours">Most hours</option>
	</select>
</div>

{#if shipped.length}
	<div class="grid">
		{#each shipped as project}<ProjectCard {project} showOwner />{/each}
	</div>
{:else}
	<div class="card dashed"><p class="muted">No shipped projects match that search.</p></div>
{/if}

<div style="margin-top: 24px">
	<Note from="Macondo / Shipwrecked">Only shipped projects appear here, so the gallery doubles as a "what a good ship looks like" reference. A devlog feed tab (Flavortown) could come later.</Note>
</div>
