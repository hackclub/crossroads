<script>
	import { page } from '$app/state';
	import Steps from '#lib/components/Steps.svelte';
	import Img from '#lib/components/Img.svelte';
	import Note from '#lib/components/Note.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, findProject, shipBlockers } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	const project = $derived(findProject(d, page.params.id));
	const blockers = $derived(project ? shipBlockers(project, d.me) : []);
	const reship = $derived(project?.ships.length > 0);
	const newHours = $derived(project ? Math.max(project.trackedHours - project.approvedHours, 0) : 0);

	const steps = ['Requirements', 'Check details', 'Tell reviewers', 'Agree'];
	let step = $state(0);
	let agreed = $state([false, false, false]);

	const reqs = $derived([
		['Public code repository', !!project?.codeUrl],
		['Demo people can try', !!project?.demoUrl],
		['README that explains how to run it', !!project?.readmeUrl],
		['Screenshot or banner', !!project?.hasBanner],
		['Description (30+ characters)', (project?.description?.length ?? 0) >= 30],
		['Linked to Hackatime', !!project?.hackatime.length],
		[`At least 3h tracked since the last ship (${newHours}h)`, newHours >= 3],
		['Profile complete (ID, address, birthday)', d.me.profileComplete]
	]);
</script>

<div class="main narrow" style="padding: 0">
	{#if !project}
		<h1>Project not found</h1>
	{:else}
		<div class="crumbs"><a href="/projects">My projects</a> / <a href="/projects/{project.id}">{project.title}</a> / {reship ? 'Ship again' : 'Ship'}</div>
		<div class="page-head"><h1>{reship ? 'Ship an update' : 'Ship'} {project.title}</h1></div>

		<div class="stack">
			{#if step < steps.length}<Steps {steps} current={step} />{/if}

			{#if step === 0}
				<section class="card">
					<h2>Ready to ship?</h2>
					<p class="small muted">A reviewer will try your project and approve the hours you tracked on it.</p>
					<ul class="checklist">
						{#each reqs as [label, ok]}
							<li><span class={ok ? 'ok' : 'no'}>{ok ? '✓' : '✗'}</span>{label}</li>
						{/each}
					</ul>
					{#if blockers.length}
						<p class="callout warn small">Fix the items marked ✗ first. <a href="/projects/{project.id}/edit">Edit project</a></p>
					{/if}
					<div class="row">
						<button class="btn primary" disabled={blockers.length > 0} onclick={() => (step = 1)}>Continue</button>
						{#if blockers.length}<button class="link-btn small muted" onclick={() => (step = 1)}>(blockout: continue anyway)</button>{/if}
					</div>
					<Note from="Flavortown / Beest">Pass/fail checklist so nobody wonders why they can't ship.</Note>
				</section>
			{:else if step === 1}
				<section class="card">
					<h2>Check your details</h2>
					<div class="grid-halves">
						<Img label="Banner" />
						<div class="stack-sm small">
							<strong>{project.title}</strong>
							<span class="muted">{project.type}</span>
							<span>{project.description}</span>
						</div>
					</div>
					<table>
						<tbody>
							<tr><th>Code</th><td>{project.codeUrl || '—'}</td></tr>
							<tr><th>Demo</th><td>{project.demoUrl || '—'}</td></tr>
							<tr><th>README</th><td>{project.readmeUrl || '—'}</td></tr>
							<tr><th>Hours this ship</th><td>{newHours}h tracked since the last approved ship</td></tr>
							<tr><th>Devlogs</th><td>{project.devlogs.length} total</td></tr>
						</tbody>
					</table>
					<div class="row">
						<button class="btn ghost" onclick={() => (step = 0)}>Back</button>
						<a class="btn ghost" href="/projects/{project.id}/edit">Edit details</a>
						<button class="btn primary" onclick={() => (step = 2)}>Looks good</button>
					</div>
				</section>
			{:else if step === 2}
				<section class="card">
					<h2>Tell the reviewers</h2>
					{#if reship}
						<label class="field"><span>What changed since your last ship? <em>500 max</em></span><textarea maxlength="500" placeholder="New features, fixes, what you learned."></textarea></label>
					{:else}
						<label class="field"><span>What did you make? <em>500 max</em></span><textarea maxlength="500" placeholder="What it does, what was hard, what you're proud of."></textarea></label>
					{/if}
					<label class="field">
						<span>How should a reviewer test it? <em>optional</em></span>
						<textarea placeholder="e.g. Log in with demo / demo, then try the export button."></textarea>
					</label>
					<label class="check"><input type="checkbox" checked={!!project.aiUse} /> I used AI on this project and declared how on the project</label>
					<div class="row">
						<button class="btn ghost" onclick={() => (step = 1)}>Back</button>
						<button class="btn primary" onclick={() => (step = 3)}>Continue</button>
					</div>
					<Note from="Flavortown">Review instructions save reviewers time and cut "couldn't run it" rejections.</Note>
				</section>
			{:else if step === 3}
				<section class="card">
					<h2>Agreements</h2>
					<label class="check"><input type="checkbox" bind:checked={agreed[0]} /> My Hackatime hours are real: no bots, scripts or idle time.</label>
					<label class="check"><input type="checkbox" bind:checked={agreed[1]} /> I made this project myself, and my README isn't AI-written.</label>
					<label class="check"><input type="checkbox" bind:checked={agreed[2]} /> I understand faking hours gets me removed from Crossroads.</label>
					<div class="row">
						<button class="btn ghost" onclick={() => (step = 2)}>Back</button>
						<button class="btn primary" disabled={!agreed.every(Boolean)} onclick={() => (step = 4)}>Submit for review</button>
					</div>
					<Note from="Horizons">Integrity agreement as the last step.</Note>
				</section>
			{:else}
				<section class="card" style="text-align: center; align-items: center">
					<h2>Shipped! 🚂</h2>
					<p>{project.title} is in the review queue. Reviews usually take about a week.</p>
					<p class="small muted">A reviewer may approve fewer hours than you tracked, and will tell you why. You'll get a Slack message either way.</p>
					<div class="row"><a class="btn primary" href="/projects/{project.id}">Back to project</a><a class="btn ghost" href="/home">Home</a></div>
				</section>
			{/if}
		</div>
	{/if}
</div>
