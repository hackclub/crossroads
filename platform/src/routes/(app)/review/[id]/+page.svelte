<script>
	// One ship, at whatever stage it's at: claim it, read the evidence and the
	// timeline, and decide. Every decision sends the stage this page loaded,
	// so a ship that moved on in the meantime is refused instead of decided
	// at the wrong stage.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api, get } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import { hours, date } from '#lib/format.js';
	import { STAGES, canDecideStage, reasonLabel, STATUS_LABELS, statusTone, EVENT_LABELS, time, toSeconds, toHours } from '#lib/review.js';
	import CodeActivity from '#lib/review/CodeActivity.svelte';

	let d = $state(null);
	let error = $state('');
	let templates = $state([]);

	// The decision form.
	let action = $state('approve');
	let approvedHours = $state(0);
	let feedback = $state('');
	let notes = $state('');
	let sendToFraud = $state(false);
	// Approving more hours than Hackatime tracked, with a reason.
	let overriding = $state(false);
	let overrideReason = $state('');
	let busy = $state(false);
	let formError = $state('');
	let result = $state(null);

	let claimError = $state('');
	let comment = $state({ message: '', internal: true });
	let commentError = $state('');
	let noteDraft = $state(null); // null when not editing the reviewer note
	let noteError = $state('');

	const id = $derived(page.params.id);
	const sub = $derived(d?.submission);
	const stage = $derived(d?.stage || null); // review, fraud, hq; null once decided
	const mine = $derived(d?.claim && d.claim.user_id === session.me.id);
	// This ship's own events since it was last sent back: what the current
	// approval chain is made of.
	const shipEvents = $derived.by(() => {
		if (!d) return [];
		const evs = d.events.filter((e) => e.submission_id === sub.id);
		const lastBack = evs.findLastIndex((e) => e.kind === 'sent_back');
		return evs.slice(lastBack + 1);
	});
	const approval = $derived(shipEvents.findLast((e) => e.kind === 'approval'));
	const fraudPass = $derived(shipEvents.findLast((e) => e.kind === 'fraud_pass'));
	const failing = $derived(d?.checks.filter((c) => c.status !== 'pass') ?? []);
	const passing = $derived(d?.checks.filter((c) => c.status === 'pass') ?? []);
	// The most hours this stage can approve.
	const maxSeconds = $derived(!sub ? 0 : stage === 'fraud' ? sub.approved_seconds : sub.new_seconds);
	// The fraud check can only cut, so only the first review and HQ can go over.
	const canOverride = $derived(stage === 'review' || stage === 'hq');

	async function load() {
		error = '';
		try {
			d = await get(`/review/submissions/${id}`);
			const start = d.stage === 'review' ? d.submission.new_seconds : (d.submission.approved_seconds ?? d.submission.new_seconds);
			approvedHours = toHours(start);
			// At HQ, a reviewer's override carries over until HQ changes it.
			overriding = d.stage === 'hq' && !!d.submission.override_reason;
			overrideReason = overriding ? d.submission.override_reason : '';
		} catch (e) {
			error = e.message;
		}
	}
	$effect(() => {
		id;
		result = null;
		action = 'approve';
		feedback = notes = '';
		sendToFraud = false;
		overriding = false;
		overrideReason = '';
		load();
	});
	onMount(() => {
		get('/review/templates').then((t) => (templates = t)).catch(() => {});
	});

	async function claim() {
		claimError = '';
		try {
			await api('POST', `/review/submissions/${id}/claim`);
			await load();
		} catch (e) {
			claimError = e.message;
		}
	}
	async function release() {
		claimError = '';
		try {
			await api('DELETE', `/review/submissions/${id}/claim`);
			await load();
		} catch (e) {
			claimError = e.message;
		}
	}
	async function skip() {
		claimError = '';
		try {
			await api('POST', `/review/submissions/${id}/skip`);
			goto(`/review?stage=${stage ?? 'review'}`);
		} catch (e) {
			claimError = e.message;
		}
	}

	let nextMsg = $state('');
	async function next(s) {
		nextMsg = '';
		try {
			const res = await api('POST', '/review/next', { stage: s });
			if (res) goto(`/review/${res.submission_id}`);
			else nextMsg = 'Nothing left in this queue for you.';
		} catch (e) {
			nextMsg = e.message;
		}
	}

	function useTemplate(e) {
		const t = templates.find((t) => t.id === +e.currentTarget.value);
		if (t) feedback = feedback ? feedback.trimEnd() + '\n\n' + t.body : t.body;
		e.currentTarget.value = '';
	}

	async function decide(e) {
		e.preventDefault();
		formError = '';
		const body = { action, stage: sub.status, feedback: feedback || undefined, internal_notes: notes || undefined };
		if (action === 'approve') {
			const asked = Math.round(Number(approvedHours) * 3600);
			if (!overriding && asked > maxSeconds) {
				formError = canOverride
					? `That's more than the ${hours(maxSeconds)} Hackatime tracked. Tick "Approve more than Hackatime tracked" and say why.`
					: `The fraud check can only lower the hours: at most ${hours(maxSeconds)}.`;
				return;
			}
			if (overriding && asked > sub.new_seconds && !overrideReason.trim()) {
				formError = 'Say why you\'re approving more than Hackatime tracked.';
				return;
			}
			body.approved_seconds = overriding ? Math.max(0, asked) : toSeconds(approvedHours, maxSeconds);
			if (overriding && body.approved_seconds > sub.new_seconds) body.override_reason = overrideReason.trim();
			const before = stage === 'review' ? sub.new_seconds : sub.approved_seconds;
			const cut = before - body.approved_seconds;
			// Cut time is gone for good, so make sure it's on purpose.
			if (cut > 0 && !confirm(`You're approving ${hours(body.approved_seconds)} of ${hours(before)}. The ${hours(cut)} you cut can't be shipped again. Go ahead?`)) return;
			if (stage === 'review' && sendToFraud) body.send_to_fraud = true;
		}
		busy = true;
		try {
			result = await api('POST', `/review/submissions/${sub.id}`, body);
			result.stage = stage;
			feedback = notes = '';
			await load();
		} catch (err) {
			formError = err.message;
			if (err.status === 409) await load();
		} finally {
			busy = false;
		}
	}

	async function postComment(e) {
		e.preventDefault();
		commentError = '';
		try {
			await api('POST', `/review/submissions/${id}/comments`, comment);
			comment = { message: '', internal: comment.internal };
			await load();
		} catch (err) {
			commentError = err.message;
		}
	}

	async function saveNote() {
		noteError = '';
		try {
			const res = await api('PUT', `/review/users/${d.user.id}/note`, { note: noteDraft });
			d.user.reviewer_note = res.note;
			noteDraft = null;
		} catch (err) {
			noteError = err.message;
		}
	}

	// What approving does at each stage, in words.
	const approveWord = $derived(stage === 'fraud' ? 'Pass' : stage === 'hq' ? 'Sign off' : 'Approve');
	const whatNext = $derived.by(() => {
		if (stage === 'review') {
			const over = overriding && Number(approvedHours) * 3600 > sub.new_seconds;
			if (sendToFraud) return 'It goes to the fraud check, then HQ.';
			if (over && !failing.some((c) => c.fraud)) return 'Approving more than Hackatime tracked always waits for HQ sign-off, even if you have it yourself. Stamps are paid then.';
			return failing.some((c) => c.fraud)
				? 'The automatic checks flagged it, so it goes to the fraud check, then HQ.'
				: session.me.can_hq_review
					? "Unless the fraud check is on for every ship, you have HQ sign-off, so this approval is final: stamps are paid right away."
					: 'Unless the fraud check is on for every ship, it goes to HQ for sign-off. Stamps are paid then.';
		}
		if (stage === 'fraud') return 'Passing sends it to HQ for sign-off. You can only lower the hours.';
		return 'Signing off pays the stamps and boxes, and sends the ship to the Unified YSWS DB.';
	});
</script>

<div class="crumbs"><a href="/review?stage={stage ?? 'review'}">Review</a> / Ship #{id}</div>

{#if error}
	<p class="callout warn">{error}</p>
{:else if d}
	{@const p = d.project}
	<div class="stack-lg">
		<div class="page-head" style="margin: 0">
			<div class="stack-sm">
				<h1>{p.name}</h1>
				<span class="muted">by {d.user.name} ({d.user.email}{d.user.slack_id ? `, Slack ${d.user.slack_id}` : ''}) · shipped {date(sub.submitted_at)}</span>
			</div>
			<span class="badge {statusTone(sub.status)}">{STATUS_LABELS[sub.status] ?? sub.status}</span>
		</div>

		{#if result}
			<section class="callout good stack-sm" role="status">
				<strong>Saved. It's now: {STATUS_LABELS[d.submission.status] ?? d.submission.status}.</strong>
				{#if result.stamps_paid || result.boxes_granted.length}
					<span>Paid {result.stamps_paid} stamps{result.boxes_granted.length ? ` and gave ${result.boxes_granted.length} box(es)` : ''}.</span>
				{/if}
				{#if result.stage && canDecideStage(session.me, result.stage)}
					<div class="row"><button class="btn small primary" onclick={() => next(result.stage)}>Next in {STAGES[result.stage].label.toLowerCase()}</button></div>
				{/if}
			</section>
		{/if}
		{#if nextMsg}<p class="callout info small" role="status">{nextMsg}</p>{/if}

		{#if stage}
			<section class="card claim-bar">
				<div class="row between">
					{#if mine}
						<span>You have this ship until {time(d.claim.expires_at)}. Nobody else can decide it until then.</span>
						<div class="row">
							<button class="btn small ghost" onclick={claim}>Keep it longer</button>
							<button class="btn small ghost" onclick={release}>Let go</button>
							<button class="btn small ghost" onclick={skip}>Skip</button>
						</div>
					{:else if d.claim}
						<span>{d.claim.name} is reviewing this until {time(d.claim.expires_at)}.</span>
					{:else}
						<span class="muted">Nobody is reviewing this right now.</span>
						{#if d.can_decide}
							<div class="row">
								<button class="btn small primary" onclick={claim}>Claim it</button>
								<button class="btn small ghost" onclick={skip}>Skip</button>
							</div>
						{/if}
					{/if}
				</div>
				{#if claimError}<p class="callout warn small">{claimError}</p>{/if}
			</section>
		{/if}

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

		<div class="grid-2">
			<div class="stack">
				{#if stage && d.can_decide}
					<form class="card" onsubmit={decide}>
						<h2>{STAGES[stage].label}</h2>
						{#if stage !== 'review'}
							<div class="chain small">
								{#if approval}<p>{approval.actor?.name ?? 'A reviewer'} approved <strong>{hours(approval.seconds)}</strong> of {hours(sub.new_seconds)} · {date(approval.created_at)}</p>{/if}
								{#if approval?.internal}<p class="muted pre">Their notes: {approval.internal}</p>{/if}
								{#if sub.override_reason}
									<p class="override">Approved for <strong>more than Hackatime tracked</strong> ({hours(sub.approved_seconds)} of {hours(sub.new_seconds)}): {sub.override_reason}</p>
								{/if}
								{#if sub.fraud_reasons?.length}
									<p>Sent to the fraud check for: {#each sub.fraud_reasons as r, i}{i ? ', ' : ''}<span class="badge warn">{reasonLabel(r)}</span>{/each}</p>
								{/if}
								{#if fraudPass}
									<p>{fraudPass.actor?.name} {fraudPass.seconds < approval?.seconds ? `cut it to ${hours(fraudPass.seconds)}` : 'passed it'} in the fraud check{fraudPass.internal ? `: ${fraudPass.internal}` : ''}</p>
								{:else if stage === 'hq'}
									<p class="muted">No fraud check: nothing was flagged.</p>
								{/if}
							</div>
						{/if}
						<div class="row">
							<label class="check"><input type="radio" bind:group={action} value="approve" /> {approveWord}</label>
							<label class="check"><input type="radio" bind:group={action} value="reject" /> Reject</label>
							{#if stage !== 'review'}<label class="check"><input type="radio" bind:group={action} value="send_back" /> Send back to review</label>{/if}
						</div>
						{#if action === 'approve'}
							<label class="field">
								<span>Hours</span>
								<input type="number" min="0" step="0.01" max={overriding ? undefined : maxSeconds / 3600} bind:value={approvedHours} />
								<small>{overriding ? `Hackatime tracked ${hours(sub.new_seconds)}.` : `Up to ${hours(maxSeconds)}.`} {stage === 'fraud' ? 'Lower it to cut hours. ' : ''}Time you cut is gone for good: it can't be shipped again.</small>
							</label>
							{#if canOverride}
								<label class="check small">
									<input type="checkbox" bind:checked={overriding} />
									Approve more than Hackatime tracked ({hours(sub.new_seconds)})
								</label>
								{#if overriding}
									<label class="field">
										<span>Why more than Hackatime? <em>required to go over</em></span>
										<textarea bind:value={overrideReason} required={Number(approvedHours) * 3600 > sub.new_seconds} placeholder="For example: 3 hours soldering and testing the board, which Hackatime can't track. The build log has photos."></textarea>
										<small>HQ sees this before signing off, and it goes to the Unified YSWS DB. The participant doesn't see it.</small>
									</label>
								{/if}
							{/if}
							{#if stage === 'review'}
								<label class="check small"><input type="checkbox" bind:checked={sendToFraud} /> Send it to the fraud check anyway</label>
							{/if}
							<p class="small muted">{whatNext}</p>
						{/if}
						{#if action !== 'send_back'}
							<label class="field">
								<span>
									Feedback for them
									<em>{action === 'reject' ? 'required' : 'optional'}</em>
								</span>
								{#if templates.length}
									<select onchange={useTemplate} aria-label="Insert a feedback template">
										<option value="">Insert a template…</option>
										{#each templates as t}<option value={t.id}>{t.title}</option>{/each}
									</select>
								{/if}
								<textarea bind:value={feedback} required={action === 'reject'}></textarea>
								<small>They see this on their project page.</small>
							</label>
						{/if}
						<label class="field">
							<span>
								{action === 'send_back' ? 'Why, for the reviewer who picks it up' : stage === 'review' ? 'Justification' : 'Notes'}
								<em>{action === 'send_back' ? 'required' : 'reviewers only'}</em>
							</span>
							<textarea bind:value={notes} required={action === 'send_back'}></textarea>
							{#if stage === 'review' && action === 'approve'}<small>Why these hours: what you checked. This goes into the Unified YSWS justification.</small>{/if}
						</label>
						{#if formError}<p class="callout warn small">{formError}</p>{/if}
						<div class="row">
							<button class="btn primary" disabled={busy}>
								{busy ? 'Saving…' : action === 'approve' ? approveWord : action === 'reject' ? 'Reject' : 'Send back'}
							</button>
						</div>
					</form>
				{:else if stage}
					<p class="callout info small">{d.cant_decide_reason}</p>
				{/if}

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

				<CodeActivity github={d.github} githubError={d.github_error} days={d.days} windowStart={d.window_start} repoUrl={p.repo_url}
					reship={p.submissions.some((s) => s.id < sub.id && s.status === 'approved')} />

				<section class="card">
					<h2>Timeline</h2>
					<ol class="timeline">
						{#each d.events as ev (ev.id)}
							{@const hidden = ev.kind === 'internal_comment' || ev.kind === 'flagged' || (!ev.message && ev.internal)}
							<li class:internal={hidden} class:other={ev.submission_id !== sub.id}>
								<div class="row between small">
									<span>
										{#if ev.kind === 'flagged'}<strong>{EVENT_LABELS.flagged}</strong>{:else}<strong>{ev.actor?.name ?? 'Someone'}</strong> {EVENT_LABELS[ev.kind] ?? ev.kind}{/if}
										{#if ev.seconds != null} · {hours(ev.seconds)}{/if}
										<span class="muted"> · ship #{ev.submission_id}</span>
									</span>
									<span class="muted">{date(ev.created_at)}{ev.edited_at ? ' (edited)' : ''}</span>
								</div>
								{#if ev.message}<p><span class="badge info tiny">They see this</span> {ev.message}</p>{/if}
								{#if ev.internal}<p class="pre"><span class="badge tiny">Reviewers only</span> {ev.internal}</p>{/if}
							</li>
						{:else}
							<li class="muted">Nothing yet.</li>
						{/each}
					</ol>
					<form class="stack" onsubmit={postComment}>
						<label class="field">
							<span>Comment</span>
							<textarea bind:value={comment.message} style="min-height: 80px" placeholder={comment.internal ? 'A note for other reviewers' : 'A message for them, shown on their project page'}></textarea>
						</label>
						<div class="row between">
							<label class="check small"><input type="checkbox" bind:checked={comment.internal} /> Only reviewers can see this</label>
							<button class="btn small" disabled={!comment.message.trim()}>{comment.internal ? 'Add note' : 'Send to them'}</button>
						</div>
						{#if commentError}<p class="callout warn small">{commentError}</p>{/if}
					</form>
				</section>

				<section class="card">
					<h2>All ships</h2>
					<ul class="list small">
						{#each p.submissions as s}
							<li class="stack-sm">
								<strong>
									{#if s.id === sub.id}This ship{:else}<a href="/review/{s.id}">#{s.id}</a>{/if}
									· {STATUS_LABELS[s.status] ?? s.status} · {date(s.submitted_at)}
								</strong>
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
					<p><strong>{hours(sub.new_seconds)}</strong> claimed by this ship</p>
					{#if sub.approved_seconds != null}<p><strong>{hours(sub.approved_seconds)}</strong> approved so far</p>{/if}
					<p class="small muted">Project's Hackatime total when shipped: {hours(sub.hackatime_seconds)} · approved on earlier ships: {hours(p.approved_seconds)}</p>
					{#if d.hackatime}
						<hr />
						<span class="small muted">Hackatime now</span>
						<ul class="list small">
							{#each d.hackatime.projects as h}<li><span style="flex: 1">{h.name}</span>{hours(h.seconds)}</li>{/each}
						</ul>
					{/if}
				</section>

				<section class="card">
					<h2>Checks</h2>
					{#each failing as c}
						<div class="check-row {c.severity === 'info' ? 'info' : 'bad'}">
							<strong>{c.status === 'inconclusive' ? "Couldn't check: " : ''}{c.name}</strong>
							{#if c.summary}<span class="small">{c.summary}</span>{/if}
							{#if c.fraud}<span class="small muted">Sends an approval to the fraud check.</span>{/if}
						</div>
					{/each}
					{#if passing.length}
						<details>
							<summary class="small" class:ok={failing.length === 0}>{failing.length === 0 ? `All ${passing.length} passed` : `${passing.length} passed`}</summary>
							<ul class="small">{#each passing as c}<li>{c.name}</li>{/each}</ul>
						</details>
					{/if}
					<p class="small muted">Run just now. The fraud check also gets every ship when an admin turns that on.</p>
				</section>

				<section class="card">
					<h2>About them</h2>
					<p class="small">Identity: {d.user.verification_status}{d.user.ysws_eligible ? ', YSWS-eligible' : ', not YSWS-eligible yet'}</p>
					{#if stage === 'fraud' && d.telescreen_url}
						<div class="row"><a class="btn small" href={d.telescreen_url} target="_blank" rel="noreferrer">Telescreen profile ↗</a></div>
					{/if}
					{#if noteDraft === null}
						{#if d.user.reviewer_note}<p class="pre small">{d.user.reviewer_note}</p>{:else}<p class="small muted">No reviewer note on this person.</p>{/if}
						<div class="row"><button class="link-btn small" onclick={() => (noteDraft = d.user.reviewer_note ?? '')}>{d.user.reviewer_note ? 'Edit note' : 'Add a note'}</button></div>
					{:else}
						<label class="field">
							<span>Reviewer note <em>reviewers only</em></span>
							<textarea bind:value={noteDraft} style="min-height: 100px"></textarea>
							<small>Shown on all of this person's ships.</small>
						</label>
						<div class="row">
							<button class="btn small primary" onclick={saveNote}>Save</button>
							<button class="btn small ghost" onclick={() => (noteDraft = null)}>Cancel</button>
						</div>
						{#if noteError}<p class="callout warn small">{noteError}</p>{/if}
					{/if}
				</section>
			</aside>
		</div>
	</div>
{:else}
	<p class="muted">Loading… (this asks Hackatime, so it can take a moment)</p>
{/if}

<style>
	.claim-bar { padding-block: 12px; }
	.chain { display: flex; flex-direction: column; gap: 6px; padding: 12px 16px; border-radius: 10px; background: rgb(0 0 0 / 0.3); }
	.timeline { list-style: none; padding: 0; display: flex; flex-direction: column; gap: 10px; }
	.timeline li { display: flex; flex-direction: column; gap: 4px; padding: 10px 14px; border-radius: 10px; background: rgb(0 0 0 / 0.25); }
	.timeline li.internal { background: transparent; border: 2px dashed var(--deep); }
	.timeline li.other { opacity: 0.7; }
	.pre { white-space: pre-wrap; }
	.override { padding: 8px 12px; border: 1px solid var(--warn); background: var(--warn-bg); border-radius: 6px; }
	.tiny { font-size: 17px; padding: 0 8px; }
	.check-row { display: flex; flex-direction: column; gap: 2px; padding: 8px 12px; border: 1px solid var(--warn); background: var(--warn-bg); border-radius: 6px; }
	.check-row.info { border-color: var(--info); background: var(--info-bg); }
</style>
