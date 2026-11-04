<script>
	import { page } from '$app/state';
	import ProjectCard from '#lib/components/ProjectCard.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, personByUsername, PROGRAM } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	const person = $derived(personByUsername(d, page.params.username));
	const projects = $derived(d.allProjects.filter((p) => p.owner === page.params.username && (person?.you || p.status === 'approved')));
</script>

{#if !person}
	<h1>No one called @{page.params.username}</h1>
{:else}
	<div class="stack-lg">
		<header class="row" style="gap: 20px">
			<span class="avatar lg">{person.name[0]}</span>
			<div class="stack-sm">
				<h1>{person.name}</h1>
				<span class="muted">@{person.username} · {person.country}</span>
				{#if person.approvedHours >= PROGRAM.goalHours}<span class="badge good">Qualified for the trip</span>{/if}
			</div>
			{#if person.you}<a class="btn ghost" href="/settings" style="margin-left: auto">Edit profile</a>{/if}
		</header>

		<div class="grid" style="grid-template-columns: repeat(auto-fill, minmax(140px, 1fr))">
			<div class="card"><span class="big">{person.approvedHours}h</span><span class="small muted">approved</span></div>
			<div class="card"><span class="big">{person.ships}</span><span class="small muted">ships</span></div>
			<div class="card"><span class="big">{projects.length}</span><span class="small muted">projects</span></div>
		</div>

		<section class="stack">
			<h2>{person.you ? 'Your projects' : 'Shipped projects'}</h2>
			{#if projects.length}
				<div class="grid">{#each projects as project}<ProjectCard {project} />{/each}</div>
			{:else}
				<p class="muted">Nothing shipped yet.</p>
			{/if}
		</section>
	</div>
{/if}
