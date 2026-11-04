<script>
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, get } from '#lib/api.js';
	import { hours, date } from '#lib/format.js';

	let d = $state(null);
	let error = $state('');
	let result = $state(null);
	let decision = $state('approved');
	let approvedHours = $state(0);
	let feedback = $state('');
	let notes = $state('');
	let busy = $state(false);
	let formError = $state('');

	async function load() {
		try {
			d = await get(`/review/submissions/${page.params.id}`);
			approvedHours = +(d.submission.new_seconds / 3600).toFixed(2);
		} catch (e) {
			error = e.message;
		}
	}
	onMount(load);

	async function decide(e) {
		e.preventDefault();
		formError = '';
		busy = true;
		const body = { status: decision, feedback: feedback || undefined, internal_notes: notes || undefined };
		if (decision === 'approved') {
			body.approved_seconds = Math.min(Math.round(approvedHours * 3600), d.submission.new_seconds);
			// Cut time is gone for good, so make sure it's on purpose.
			const cut = d.submission.new_seconds - body.approved_seconds;
			if (cut > 0 && !confirm(`You're approving ${hours(body.approved_seconds)} of the ${hours(d.submission.new_seconds)} claimed. The ${hours(cut)} you cut can't be shipped again. Approve?`)) {
				busy = false;
				return;
			}
		}
		try {
			result = await api('POST', `/review/submissions/${d.submission.id}`, body);
			await load();
		} catch (err) {
			formError = err.message;
		} finally {
			busy = false;
		}
	}
</script>

<div class="crumbs"><a href="/review">Review queue</a> / Submission #{page.params.id}</div>

{#if error}
	<p class="callout warn">{error}</p>
{:else if d}
	{@const p = d.project}
	<div class="stack-lg">
		<div class="page-head" style="margin: 0">
			<div class="stack-sm">
				<h1>{p.name}</h1>
				<span class="muted">by {d.user.name} ({d.user.email}{d.user.slack_id ? `, Slack ${d.user.slack_id}` : ''}) · submitted {date(d.submission.submitted_at)}</span>
			</div>
			<span class="badge {d.submission.status === 'pending' ? 'info' : d.submission.status === 'approved' ? 'good' : 'warn'}">{d.submission.status}</span>
		</div>

		{#if d.projects_sharing_links.length}
			<section class="callout warn">
				<strong>Other projects use the same repo or demo link</strong>
				<ul class="small">
					{#each d.projects_sharing_links as o}<li>{o.name} by {o.user_name} · {o.repo_url ?? ''} {o.demo_url ?? ''}</li>{/each}
				</ul>
			</section>
		{/if}
		{#if d.hackatime?.convicted}<p class="callout warn"><strong>Hackatime has flagged this person as cheating.</strong></p>{/if}
		{#if d.hackatime_error}<p class="callout warn">Hackatime: {d.hackatime_error}</p>{/if}
		{#if !d.user.ysws_eligible}<p class="callout info small">Identity: {d.user.verification_status}, not YSWS-eligible yet. They can still ship, but can't buy until verified.</p>{/if}

		{#if result}
			<p class="callout good" role="status">Saved. Paid {result.stamps_paid} stamps{result.boxes_granted.length ? ` and granted ${result.boxes_granted.length} box(es)` : ''}.</p>
		{/if}

		<div class="grid-2">
			<div class="stack">
				<section class="card">
					<h2>Project</h2>
					<p style="white-space: pre-wrap">{p.description}</p>
					<div class="row small">
						{#if p.demo_url}<a class="chip" href={p.demo_url} target="_blank" rel="noreferrer">▶ Demo</a>{/if}
						{#if p.repo_url}<a class="chip" href={p.repo_url} target="_blank" rel="noreferrer">Repo</a>{/if}
						{#if p.screenshot_url}<a class="chip" href={p.screenshot_url} target="_blank" rel="noreferrer">Screenshot</a>{/if}
					</div>
					{#if p.screenshot_url}<img src={p.screenshot_url} alt="Screenshot" style="max-height: 280px; object-fit: contain; align-self: flex-start" />{/if}
				</section>

				{#if d.submission.status === 'pending'}
					<form class="card" onsubmit={decide}>
						<h2>Decision</h2>
						<div class="row">
							<label class="check"><input type="radio" bind:group={decision} value="approved" /> Approve</label>
							<label class="check"><input type="radio" bind:group={decision} value="rejected" /> Reject</label>
						</div>
						{#if decision === 'approved'}
							<label class="field">
								<span>Hours to approve <em>up to {hours(d.submission.new_seconds)}</em></span>
								<input type="number" min="0" step="0.01" max={d.submission.new_seconds / 3600} bind:value={approvedHours} style="max-width: 160px" />
								<small>Time you cut is gone for good: it can't be shipped again.</small>
							</label>
						{/if}
						<label class="field">
							<span>Feedback for them <em>{decision === 'rejected' ? 'required' : 'optional'}</em></span>
							<textarea bind:value={feedback} required={decision === 'rejected'}></textarea>
						</label>
						<label class="field">
							<span>Internal notes <em>reviewers only</em></span>
							<textarea bind:value={notes}></textarea>
						</label>
						{#if formError}<p class="callout warn small">{formError}</p>{/if}
						<div class="row"><button class="btn primary" disabled={busy}>{busy ? 'Saving…' : decision === 'approved' ? 'Approve' : 'Reject'}</button></div>
					</form>
				{/if}

				<section class="card">
					<h2>Earlier ships</h2>
					<ul class="list small">
						{#each p.submissions as s}
							<li class="stack-sm">
								<strong>#{s.id} · {s.status} · {date(s.submitted_at)}</strong>
								<span class="muted">Claimed {hours(s.new_seconds)}{s.approved_seconds != null ? ` · approved ${hours(s.approved_seconds)}` : ''}</span>
								{#if s.feedback}<span>Feedback: {s.feedback}</span>{/if}
								{#if s.internal_notes}<span class="muted">Notes: {s.internal_notes}</span>{/if}
							</li>
						{/each}
					</ul>
				</section>
			</div>

			<aside class="stack">
				<section class="card">
					<h2>Hours</h2>
					<p><strong>{hours(d.submission.new_seconds)}</strong> claimed by this ship</p>
					<p class="small muted">Project total when submitted: {hours(d.submission.hackatime_seconds)} · approved so far: {hours(p.approved_seconds)}</p>
					{#if d.hackatime}
						<hr />
						<span class="small muted">Hackatime now</span>
						<ul class="list small">
							{#each d.hackatime.projects as h}<li><span style="flex: 1">{h.name}</span>{hours(h.seconds)}</li>{/each}
						</ul>
					{/if}
				</section>
			</aside>
		</div>
	</div>
{:else}
	<p class="muted">Loading…</p>
{/if}
