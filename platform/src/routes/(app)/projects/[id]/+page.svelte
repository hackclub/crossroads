<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, get } from '#lib/api.js';
	import { refresh } from '#lib/session.svelte.js';
	import { hours, date, projectStatus } from '#lib/format.js';

	let project = $state(null);
	let stamps = $state([]);
	let error = $state('');
	let actionError = $state('');
	let stampId = $state('');
	let busy = $state(false);
	let shotFailed = $state(false);

	async function load() {
		error = '';
		try {
			[project, stamps] = await Promise.all([get(`/projects/${page.params.id}`), get('/special-stamps')]);
		} catch (e) {
			error = e.message;
		}
	}
	onMount(load);

	const latest = $derived(project?.submissions[0]);
	const isPending = $derived(latest?.status === 'pending');
	const usable = $derived(stamps.filter((s) => s.type.effect !== 'none' && s.submission_id === null));
	const missing = $derived(
		project
			? [
					!project.description && 'a description',
					!project.repo_url && 'a repo link',
					!project.demo_url && 'a demo link',
					!project.screenshot_url && 'a screenshot',
					!project.hackatime_projects.length && 'a Hackatime project'
				].filter(Boolean)
			: []
	);

	async function ship() {
		actionError = '';
		busy = true;
		try {
			await api('POST', `/projects/${project.id}/submissions`, stampId ? { special_stamp_id: Number(stampId) } : undefined);
			stampId = '';
			await load();
		} catch (e) {
			actionError = e.message;
			if (e.status === 409) await load();
		} finally {
			busy = false;
		}
	}

	async function remove() {
		if (!confirm(`Delete ${project.name}? This can't be undone.`)) return;
		try {
			await api('DELETE', `/projects/${project.id}`);
			await refresh();
			goto('/home');
		} catch (e) {
			actionError = e.message;
		}
	}
</script>

{#if error}
	<p class="callout warn">{error}</p>
	<p><a href="/home">Back home</a></p>
{:else if !project}
	<p class="muted">Loading… (this asks Hackatime, so it can take a moment)</p>
{:else}
	{@const s = projectStatus(project)}
	<div class="stack-lg">
		<div class="stack-sm" style="align-items: center">
			<h1 class="page-title" style="margin-bottom: 4px">{project.name}</h1>
			<span class="badge {s.tone}">{s.label}</span>
		</div>

		<div class="stack">
			<section class="hero-well">
				{#if project.screenshot_url && !shotFailed}
					<img src={project.screenshot_url} alt="Screenshot of {project.name}" onerror={() => (shotFailed = true)} />
				{:else}
					<span class="muted small">{project.screenshot_url ? "The screenshot didn't load." : 'No screenshot yet.'}</span>
				{/if}
				<div class="pill-row pills">
					{#if project.demo_url}<a class="chip" href={project.demo_url} target="_blank" rel="noreferrer">Demo</a>{/if}
					{#if project.repo_url}<a class="chip" href={project.repo_url} target="_blank" rel="noreferrer">Repo</a>{/if}
					<a class="chip" href="/projects/{project.id}/edit">Edit</a>
					{#if !project.submissions.length}<button class="chip" onclick={remove}>Delete</button>{/if}
				</div>
			</section>

			<section class="panel">
				<div class="panel-head">Description</div>
				<div class="panel-body"><p style="white-space: pre-wrap">{project.description || 'No description yet.'}</p></div>
			</section>

			<div class="grid-halves">
				<section class="panel">
					<div class="panel-head">Ship it</div>
					<div class="panel-body">
						<span class="small muted">A reviewer tries your project and approves the new hours you tracked on it. Approved hours pay stamps.</span>
						{#if isPending}
							<p class="callout info small">Waiting for review since {date(latest.submitted_at)}. The result shows up below.</p>
						{:else if missing.length}
							<p class="callout warn small">Before you can ship, add {missing.join(', ')}. <a href="/projects/{project.id}/edit">Edit project</a></p>
						{:else if project.hackatime_error}
							<p class="callout warn small">{project.hackatime_error}</p>
						{:else if !project.unsubmitted_seconds}
							<p class="callout info small">No new time to ship. Keep building, then ship again.</p>
						{:else}
							<p><span class="big">{hours(project.unsubmitted_seconds)}</span> of new time goes to review.</p>
							{#if usable.length}
								<label class="row small">
									Use a special stamp:
									<select bind:value={stampId} style="width: auto; background: var(--deep)">
										<option value="">None</option>
										{#each usable as st}<option value={st.id}>{st.type.name} ({st.type.effect.replace('_', ' ')})</option>{/each}
									</select>
									<span class="muted">Used up either way.</span>
								</label>
							{/if}
							<div class="row"><button class="btn primary" disabled={busy} onclick={ship}>{busy ? 'Shipping…' : project.submissions.length ? 'Ship again' : 'Ship'}</button></div>
						{/if}
						{#if actionError}<p class="callout warn small">{actionError}</p>{/if}
					</div>
				</section>

				<section class="panel">
					<div class="panel-head">Hours</div>
					<div class="panel-body">
						<div class="grid-halves" style="gap: 12px">
							<div class="tile"><span class="label">Tracked</span><span class="value" style="background: var(--deep)">{hours(project.hackatime_seconds ?? 0)}</span></div>
							<div class="tile"><span class="label">Approved</span><span class="value" style="background: var(--deep)">{hours(project.approved_seconds)}</span></div>
						</div>
						{#if project.unsubmitted_seconds != null}<span class="small">{hours(project.unsubmitted_seconds)} not shipped yet.</span>{/if}
						<div class="pill-row">
							{#each project.hackatime_projects as h}<span class="badge">{h}</span>{:else}<span class="small muted">No Hackatime projects linked. <a href="/projects/{project.id}/edit">Link some</a></span>{/each}
						</div>
						<button class="link-btn small muted" onclick={load}>Refresh from Hackatime</button>
					</div>
				</section>
			</div>
		</div>

		<section class="stack" style="align-items: center">
			<h2>Ships</h2>
			{#each project.submissions as sub, i}
				<article class="light-card ship">
					<div class="well stack-sm">
						<div class="row between"><span>Ship #{project.submissions.length - i}</span><span class="badge {sub.status === 'approved' ? 'good' : sub.status === 'rejected' ? 'warn' : 'info'}">{sub.status === 'pending' ? 'in review' : sub.status}</span></div>
						<span class="big">{hours(sub.approved_seconds ?? sub.new_seconds)}</span>
						<span class="small">{sub.approved_seconds != null ? `approved of ${hours(sub.new_seconds)} claimed` : 'claimed'} · {sub.hackatime_projects.join(', ')}</span>
					</div>
					<span class="small">{date(sub.submitted_at)}</span>
					{#if sub.feedback}<p>Reviewer: “{sub.feedback}”</p>{/if}
				</article>
			{:else}
				<p class="muted">Not shipped yet.</p>
			{/each}
		</section>
	</div>
{/if}

<style>
	/* The tall #474242 block from "project main", with the pills on it. */
	.hero-well {
		position: relative;
		display: grid;
		place-items: center;
		min-height: 290px;
		padding-bottom: 64px;
		background: var(--deep);
		border-radius: var(--radius);
		overflow: hidden;
	}
	.hero-well img { width: 100%; max-height: 520px; object-fit: contain; }
	.pills { position: absolute; left: 14px; bottom: 14px; }
	.ship { width: min(560px, 100%); }
</style>
