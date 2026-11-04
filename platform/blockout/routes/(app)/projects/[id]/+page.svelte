<script>
	import { page } from '$app/state';
	import Img from '#lib/components/Img.svelte';
	import Badge from '#lib/components/Badge.svelte';
	import Note from '#lib/components/Note.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, findProject, shipBlockers, PROGRAM } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	const project = $derived(findProject(d, page.params.id));
	const mine = $derived(project?.owner === d.me.username);
	const blockers = $derived(project && mine ? shipBlockers(project, d.me) : []);
	const latest = $derived(project?.ships?.[0]);
	const newHours = $derived(project ? Math.max((project.trackedHours ?? 0) - project.approvedHours, 0) : 0);

	// Where the latest ship is in review, for the tracker.
	const trackerStep = $derived(
		!latest ? -1 : latest.status === 'in_review' ? 1 : 2
	);

	let composing = $state(false);
</script>

{#if !project}
	<h1>Project not found</h1>
	<p><a href="/projects">Back to my projects</a></p>
{:else}
	<div class="crumbs">
		{#if mine}<a href="/projects">My projects</a>{:else}<a href="/explore">Explore</a>{/if} / {project.title}
	</div>

	<div class="stack-lg">
		<header class="stack">
			{#if project.hasBanner !== false}
				<Img label="Banner / screenshot" ratio="21 / 7" />
			{:else}
				<div class="img empty" style="aspect-ratio: 21 / 7"><span>No screenshot yet{#if mine} · <a href="/projects/{project.id}/edit">add one</a>{/if}</span></div>
			{/if}

			<div class="page-head" style="margin: 0">
				<div class="stack-sm">
					<div class="row"><h1>{project.title}</h1><Badge status={project.status} /></div>
					<div class="row small muted">
						<span>{project.type}</span><span>·</span>
						<a href="/u/{project.owner}">@{project.owner}</a><span>·</span>
						<span>{project.approvedHours}h approved</span>
					</div>
				</div>
				<div class="row">
					{#if mine}
						<a class="btn ghost" href="/projects/{project.id}/edit">Edit</a>
						{#if blockers.length}
							<button class="btn primary" disabled title="See why below">{project.ships.length ? 'Ship again' : 'Ship'}</button>
						{:else}
							<a class="btn primary" href="/projects/{project.id}/ship">{project.ships.length ? 'Ship again' : 'Ship'}</a>
						{/if}
					{:else}
						<button class="btn ghost">♥ {project.likes ?? 0}</button>
						<button class="btn ghost small">Report</button>
					{/if}
				</div>
			</div>

			<div class="row">
				{#if project.demoUrl}<a class="chip" href={project.demoUrl} target="_blank" rel="noreferrer">▶ Try it</a>{/if}
				{#if project.codeUrl}<a class="chip" href={project.codeUrl} target="_blank" rel="noreferrer">Code</a>{/if}
				{#if project.readmeUrl}<a class="chip" href={project.readmeUrl} target="_blank" rel="noreferrer">README</a>{/if}
			</div>
		</header>

		{#if mine && blockers.length && project.status !== 'in_review'}
			<section class="callout warn">
				<strong>Before you can ship:</strong>
				<ul class="small" style="margin: 6px 0 0; padding-left: 18px">
					{#each blockers as b}<li>{b}</li>{/each}
				</ul>
			</section>
		{/if}

		<div class="grid-2">
			<div class="stack">
				<section class="card">
					<h2>About</h2>
					<p>{project.description || 'No description yet.'}</p>
					{#if project.aiUse}
						<p class="small muted"><strong>AI use:</strong> {project.aiUse}</p>
					{/if}
				</section>

				{#if mine && latest}
					<section class="card">
						<div class="card-title"><h2>Latest ship</h2><span class="muted small">Ship #{latest.n} · {latest.submitted}</span></div>
						<ol class="steps">
							<li class:done={trackerStep > 0} class:current={trackerStep === 0}><span class="num">{trackerStep > 0 ? '✓' : 1}</span>Submitted</li>
							<li class:done={trackerStep > 1} class:current={trackerStep === 1}><span class="num">{trackerStep > 1 ? '✓' : 2}</span>In review</li>
							<li class:current={trackerStep === 2}><span class="num">3</span>{latest.status === 'in_review' ? 'Decision' : latest.status === 'approved' ? `Approved · +${latest.approvedHours}h` : latest.status === 'changes_needed' ? 'Changes needed' : 'Rejected'}</li>
						</ol>
						{#if latest.status === 'in_review'}
							<p class="small muted">Reviews usually take about a week. You'll get a Slack message when it's done.</p>
							<div class="row"><button class="btn small ghost">Withdraw from review</button></div>
						{/if}
						{#if latest.feedback}
							<div class="callout {latest.status === 'approved' ? 'good' : 'warn'}">
								<strong class="small">Reviewer feedback</strong>
								<p class="small">{latest.feedback}</p>
							</div>
						{/if}
						<Note from="Horizons">Arrow-style tracker for where the ship is, with feedback right under it.</Note>
					</section>
				{/if}

				<section class="stack">
					<div class="card-title">
						<h2>Devlogs</h2>
						{#if mine}<button class="btn small" onclick={() => (composing = !composing)}>{composing ? 'Cancel' : '+ Write a devlog'}</button>{/if}
					</div>
					{#if composing}
						<div class="card">
							<label class="field"><span>What did you work on?</span><textarea placeholder="What you built, what broke, what you learned."></textarea></label>
							<div class="dropzone small">Add screenshots or a video (at least one)</div>
							<p class="small muted">This devlog covers <strong>{newHours}h</strong> tracked since your last devlog.</p>
							<div class="row"><button class="btn primary" onclick={() => (composing = false)}>Post devlog</button></div>
							<Note from="Flavortown / Fallen">Hours come from Hackatime, not typed in. One devlog is needed per ship.</Note>
						</div>
					{/if}
					{#if project.devlogs?.length}
						{#each project.devlogs as log}
							<article class="card">
								<div class="row between small muted"><span>{log.date}</span><span>{log.hours}h</span></div>
								<p>{log.text}</p>
								{#if log.images}
									<div class="grid" style="grid-template-columns: repeat({log.images}, minmax(0, 160px))">
										{#each Array(log.images) as _}<Img label="Screenshot" ratio="4 / 3" />{/each}
									</div>
								{/if}
							</article>
						{/each}
					{:else}
						<div class="card dashed"><p class="muted small">No devlogs yet. {mine ? 'Post one to show your progress. You need one before you ship.' : ''}</p></div>
					{/if}
				</section>
			</div>

			<aside class="stack">
				<section class="card">
					<h2>Hours</h2>
					<div class="grid-halves">
						<div class="stack-sm"><span class="big">{project.trackedHours ?? project.approvedHours}h</span><span class="small muted">tracked</span></div>
						<div class="stack-sm"><span class="big">{project.approvedHours}h</span><span class="small muted">approved</span></div>
					</div>
					{#if mine}
						<p class="small">{newHours}h tracked since your last approved ship{newHours < PROGRAM.minShipHours ? ` (need ${PROGRAM.minShipHours}h to ship)` : ''}.</p>
						<hr />
						<div class="stack-sm">
							<span class="small muted">Linked Hackatime projects</span>
							<div class="row">
								{#each project.hackatime as h}<span class="chip">{h.name} · {h.hours}h</span>{/each}
							</div>
							<div class="row"><button class="btn small ghost">Refresh</button><a class="btn small ghost" href="/projects/{project.id}/edit">Change</a></div>
						</div>
					{/if}
				</section>

				{#if mine && project.ships.length}
					<section class="card">
						<h2>Ship history</h2>
						<ul class="list small">
							{#each project.ships as s}
								<li class="stack-sm">
									<div class="row between" style="width: 100%"><strong>Ship #{s.n}</strong><Badge status={s.status} /></div>
									<span class="muted">{s.submitted}{s.approvedHours ? ` · +${s.approvedHours}h approved` : ''}</span>
								</li>
							{/each}
						</ul>
					</section>
				{/if}

				<section class="card">
					<h2>Made by</h2>
					<a class="row" href="/u/{project.owner}"><span class="avatar">{project.owner[0].toUpperCase()}</span><strong>@{project.owner}</strong></a>
					<Note from="Macondo">Room for collaborators here if team projects are allowed.</Note>
				</section>
			</aside>
		</div>
	</div>
{/if}
