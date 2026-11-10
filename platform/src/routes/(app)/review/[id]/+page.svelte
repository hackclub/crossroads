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

{#snippet icon(state)}
	<!-- A check's state as a shape as well as a color: tick, cross, bang, question. -->
	<svg class="state {state}" viewBox="0 0 16 16" aria-hidden="true">
		{#if state === 'pass'}<path d="M3.5 8.5l3 3 6-7" />
		{:else if state === 'fail'}<path d="M4.5 4.5l7 7M11.5 4.5l-7 7" />
		{:else if state === 'warn'}<path d="M8 3.5v6M8 12.2v.3" />
		{:else}<path d="M5.8 5.8a2.3 2.3 0 1 1 3 2.2c-.6.2-.8.6-.8 1.2v.6M8 12.2v.3" />{/if}
	</svg>
{/snippet}

{#if error}
	<div class="crumbs"><a href="/review?stage=review">Review</a> / Ship #{id}</div>
	<p class="callout warn">{error}</p>
{:else if d}
	{@const p = d.project}
	<div class="review-page">
		<header class="head">
			<div class="title">
				<a class="back" href="/review?stage={stage ?? 'review'}">
					<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M10 3.5L5.5 8l4.5 4.5" /></svg>
					Queue
				</a>
				<span class="muted">#{id}</span>
				<h1>{p.name}</h1>
				<span class="by">by <strong>{d.user.name}</strong></span>
				<span class="badge {statusTone(sub.status)}">{STATUS_LABELS[sub.status] ?? sub.status}</span>
			</div>
			{#if stage}
				<div class="claim small">
					{#if mine}
						<span>Yours until {time(d.claim.expires_at)}</span>
						<button class="btn small ghost" onclick={claim}>Keep it longer</button>
						<button class="btn small ghost" onclick={release}>Let go</button>
						<button class="btn small ghost" onclick={skip}>Skip</button>
					{:else if d.claim}
						<span>{d.claim.name} is reviewing this until {time(d.claim.expires_at)}</span>
					{:else}
						<span class="muted">Nobody is reviewing this</span>
						{#if d.can_decide}
							<button class="btn small primary" onclick={claim}>Claim it</button>
							<button class="btn small ghost" onclick={skip}>Skip</button>
						{/if}
					{/if}
				</div>
			{/if}
		</header>
		{#if claimError}<p class="callout warn small">{claimError}</p>{/if}
		{#if nextMsg}<p class="callout info small" role="status">{nextMsg}</p>{/if}

		<div class="bento">
			<div class="who">
				<section class="card">
					<h2>{d.user.name}</h2>
					<dl class="facts small">
						<dt>Email</dt><dd>{d.user.email}</dd>
						<dt>Slack</dt><dd>{d.user.slack_id ?? 'Not linked'}</dd>
						<dt>Identity</dt><dd>{d.user.verification_status}{d.user.ysws_eligible ? ', YSWS-eligible' : ', not YSWS-eligible yet'}</dd>
						<dt>Shipped</dt><dd>{date(sub.submitted_at)}</dd>
					</dl>
					{#if stage === 'fraud' && d.telescreen_url}
						<a class="btn small" href={d.telescreen_url} target="_blank" rel="noreferrer">Telescreen profile</a>
					{/if}
					{#if noteDraft === null}
						{#if d.user.reviewer_note}<p class="pre small note">{d.user.reviewer_note}</p>{/if}
						<button class="link-btn small" onclick={() => (noteDraft = d.user.reviewer_note ?? '')}>{d.user.reviewer_note ? 'Edit reviewer note' : 'Add a reviewer note'}</button>
					{:else}
						<label class="field">
							<span>Reviewer note</span>
							<textarea bind:value={noteDraft} style="min-height: 90px"></textarea>
							<small>Reviewers only. Shown on all of this person's ships.</small>
						</label>
						<div class="row">
							<button class="btn small primary" onclick={saveNote}>Save</button>
							<button class="btn small ghost" onclick={() => (noteDraft = null)}>Cancel</button>
						</div>
						{#if noteError}<p class="callout warn small">{noteError}</p>{/if}
					{/if}
				</section>

				<section class="card">
					<h2>Hours</h2>
					<dl class="numbers">
						<dt>Claimed by this ship</dt><dd>{hours(sub.new_seconds)}</dd>
						{#if sub.approved_seconds != null}<dt>Approved so far</dt><dd>{hours(sub.approved_seconds)}</dd>{/if}
						<dt>Hackatime when shipped</dt><dd>{hours(sub.hackatime_seconds)}</dd>
						{#if d.hackatime}<dt>Hackatime now</dt><dd>{hours(d.hackatime.seconds)}</dd>{/if}
						<dt>Approved on earlier ships</dt><dd>{hours(p.approved_seconds)}</dd>
					</dl>
					{#if d.hackatime?.projects.length}
						<p class="small muted">{d.hackatime.projects.map((h) => `${h.name} ${hours(h.seconds)}`).join(' · ')}</p>
					{/if}
				</section>

			</div>

			<div class="checks-col">
				<section class="card">
					<div class="row between">
						<h2>Checks</h2>
						<span class="small" class:ok={failing.length === 0}>{failing.length === 0 ? `All ${passing.length} passed` : `${failing.length} to look at`}</span>
					</div>
					<ul class="checks">
						{#each failing as c (c.id)}
							{@const state = c.status === 'inconclusive' ? 'unknown' : c.severity === 'info' || c.severity === 'warn' ? 'warn' : 'fail'}
							<li class={state}>
								{@render icon(state)}
								<div>
									<strong>{c.status === 'inconclusive' ? "Couldn't check: " : ''}{c.name}</strong>
									{#if c.fraud}<span class="tag">fraud signal</span>{/if}
									{#if c.summary}<p>{c.summary}</p>{/if}
								</div>
							</li>
						{/each}
					</ul>
					{#if passing.length}
						<details open={failing.length === 0}>
							<summary class="small">{failing.length ? `${passing.length} passed` : 'Show them'}</summary>
							<ul class="checks">
								{#each passing as c (c.id)}
									<li class="pass">{@render icon('pass')}<div><span>{c.name}</span>{#if c.summary && c.id === 'repo_works'}<p>{c.summary}</p>{/if}</div></li>
								{/each}
							</ul>
						</details>
					{/if}
					<p class="small muted">Run just now. "Fraud signal" checks send an approval to the fraud check.</p>
				</section>
			</div>

			<div class="main-col">
				<section class="card project">
					{#if p.screenshot_url}
						<a class="shot" href={p.screenshot_url} target="_blank" rel="noreferrer"><img src={p.screenshot_url} alt="Screenshot of {p.name}" /></a>
					{/if}
					<div class="stack-sm about">
						<h2>Project</h2>
						<p class="pre">{p.description}</p>
						<div class="row links">
							{#if p.demo_url}<a class="btn small primary" href={p.demo_url} target="_blank" rel="noreferrer">Try the demo</a>{/if}
							{#if p.repo_url}<a class="btn small ghost" href={p.repo_url} target="_blank" rel="noreferrer">Repo</a>{/if}
						</div>
						{#if p.submissions.length > 1}
							<ul class="ships small">
								{#each p.submissions as s (s.id)}
									<li>
										{#if s.id === sub.id}<strong>This ship</strong>{:else}<a href="/review/{s.id}">#{s.id}</a>{/if}
										<span class="badge tiny {statusTone(s.status)}">{STATUS_LABELS[s.status] ?? s.status}</span>
										<span class="muted">{hours(s.new_seconds)} claimed{s.approved_seconds != null ? `, ${hours(s.approved_seconds)} approved` : ''} · {date(s.submitted_at)}</span>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
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
							<li class="muted small">Nothing yet.</li>
						{/each}
					</ol>
					<form class="stack-sm" onsubmit={postComment}>
						<label class="field">
							<span>Comment</span>
							<textarea bind:value={comment.message} style="min-height: 64px" placeholder={comment.internal ? 'A note for other reviewers' : 'A message for them, shown on their project page'}></textarea>
						</label>
						<div class="row between">
							<label class="check small"><input type="checkbox" bind:checked={comment.internal} /> Only reviewers can see this</label>
							<button class="btn small" disabled={!comment.message.trim()}>{comment.internal ? 'Add note' : 'Send to them'}</button>
						</div>
						{#if commentError}<p class="callout warn small">{commentError}</p>{/if}
					</form>
				</section>
			</div>

			<div class="decide-col">
				<div class="decide">
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
							<div class="row actions">
								<label class="check"><input type="radio" bind:group={action} value="approve" /> {approveWord}</label>
								<label class="check"><input type="radio" bind:group={action} value="reject" /> Reject</label>
								{#if stage !== 'review'}<label class="check"><input type="radio" bind:group={action} value="send_back" /> Send back</label>{/if}
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
											<span>Why more than Hackatime?</span>
											<textarea bind:value={overrideReason} required={Number(approvedHours) * 3600 > sub.new_seconds} placeholder="For example: 3 hours soldering and testing the board, which Hackatime can't track. The build log has photos."></textarea>
											<small>Required to go over. HQ sees this before signing off, and it goes to the Unified YSWS DB. The participant doesn't see it.</small>
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
									<span>Feedback for them</span>
									{#if templates.length}
										<select onchange={useTemplate} aria-label="Insert a feedback template">
											<option value="">Insert a template…</option>
											{#each templates as t}<option value={t.id}>{t.title}</option>{/each}
										</select>
									{/if}
									<textarea bind:value={feedback} required={action === 'reject'}></textarea>
									<small>{action === 'reject' ? 'Required. ' : 'Optional. '}They see this on their project page.</small>
								</label>
							{/if}
							<label class="field">
								<span>{action === 'send_back' ? 'Why, for the reviewer who picks it up' : stage === 'review' ? 'Justification' : 'Notes'}</span>
								<textarea bind:value={notes} required={action === 'send_back'}></textarea>
								<small>
									{action === 'send_back' ? 'Required. ' : 'Reviewers only. '}{stage === 'review' && action === 'approve' ? 'Why these hours: what you checked. This goes into the Unified YSWS justification.' : ''}
								</small>
							</label>
							{#if formError}<p class="callout warn small">{formError}</p>{/if}
							<button class="btn primary submit" disabled={busy}>
								{busy ? 'Saving…' : action === 'approve' ? approveWord : action === 'reject' ? 'Reject' : 'Send back'}
							</button>
						</form>
					{:else if stage}
						<p class="callout info small">{d.cant_decide_reason}</p>
					{:else if !result}
						<p class="callout info small">This ship has been decided: {STATUS_LABELS[sub.status] ?? sub.status}{sub.approved_seconds != null ? `, ${hours(sub.approved_seconds)}` : ''}.</p>
					{/if}
				</div>
			</div>
		</div>
	</div>
{:else}
	<p class="muted">Loading… (this asks Hackatime and GitHub, so it can take a moment)</p>
{/if}

<style>
	/* The review page leaves the site's reading column for most of the screen,
	   in Sidekick's arrangement: who and what's flagged on the left, the
	   project and its evidence in the middle, the decision on the right, where
	   it stays while you scroll. Narrower screens fold to two columns, then one. */
	.review-page {
		--rv-width: min(1880px, calc(100vw - 64px));
		width: var(--rv-width);
		margin-left: calc((100% - var(--rv-width)) / 2);
		font-size: 19px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.review-page :global(.small) { font-size: 17px; }
	.review-page :global(.card) { padding: 16px 20px; gap: 10px; border-radius: 18px; }
	.review-page :global(.card h2) { font-size: 28px; }
	.review-page :global(.btn.small) { font-size: 18px; padding: 2px 12px; }
	.review-page :global(.field > span) { font-size: 19px; padding: 4px 14px; }
	.review-page :global(.field input), .review-page :global(.field textarea), .review-page :global(.field select) { font-size: 19px; padding: 8px 14px; }
	.review-page :global(.field small) { font-size: 16px; }
	.review-page :global(.field textarea) { min-height: 72px; }
	.review-page :global(.callout) { margin: 0; }

	.head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 24px; }
	.title { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 12px; min-width: 0; }
	.title h1 { font-size: 40px; line-height: 1.1; overflow-wrap: anywhere; }
	.title .badge { align-self: center; }
	.back { display: inline-flex; align-items: center; gap: 2px; align-self: center; padding: 2px 12px 2px 6px; border-radius: 999px; background: rgb(0 0 0 / 0.35); box-shadow: inset 0 0 0 1px var(--head); color: var(--text); text-decoration: none; font-size: 17px; }
	.back:hover { background: var(--deep); }
	.back svg, .state { width: 16px; height: 16px; flex: none; fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
	.claim { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; }

	.bento {
		display: grid;
		gap: 12px;
		align-items: start;
		grid-template-columns: minmax(0, 1fr);
		grid-template-areas: 'checks' 'decide' 'who' 'main';
	}
	.who { grid-area: who; }
	.checks-col { grid-area: checks; }
	.main-col { grid-area: main; }
	.decide-col { grid-area: decide; align-self: stretch; }
	.who, .checks-col, .main-col, .decide { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
	/* Two columns: the evidence on the left; what's flagged, the decision and
	   the person on the right, in the order a reviewer needs them. */
	@media (min-width: 1000px) {
		.bento {
			grid-template-columns: minmax(0, 1fr) minmax(0, 420px);
			grid-template-rows: auto auto 1fr;
			grid-template-areas: 'main checks' 'main decide' 'main who';
		}
	}
	@media (min-width: 1500px) {
		.bento {
			grid-template-columns: minmax(0, 360px) minmax(0, 1fr) minmax(0, 440px);
			grid-template-rows: auto 1fr;
			grid-template-areas: 'who main decide' 'checks main decide';
		}
		.decide { position: sticky; top: 16px; max-height: calc(100vh - 32px); overflow-y: auto; scrollbar-width: thin; }
	}

	.facts { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 2px 12px; margin: 0; }
	.facts dt { color: var(--muted); }
	.facts dd { margin: 0; overflow-wrap: anywhere; }
	.note { padding: 8px 12px; border-radius: 10px; background: rgb(0 0 0 / 0.3); margin: 0; }
	.numbers { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 4px 12px; margin: 0; }
	.numbers dt { color: var(--muted); }
	.numbers dd { margin: 0; text-align: right; font-variant-numeric: tabular-nums; }
	.numbers dt:first-child, .numbers dt:first-child + dd { color: var(--text); font-size: 24px; }

	.checks { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 6px; }
	.checks li { display: flex; gap: 10px; align-items: flex-start; padding: 6px 10px; border-radius: 10px; font-size: 17px; line-height: 1.35; }
	.checks li .state { margin-top: 2px; }
	.checks li p { margin: 2px 0 0; color: var(--muted); }
	.checks li.fail { background: var(--warn-bg); box-shadow: inset 0 0 0 1px var(--danger); }
	.checks li.fail .state { color: var(--danger); }
	.checks li.warn { background: var(--warn-bg); }
	.checks li.warn .state { color: var(--warn); }
	.checks li.unknown { background: rgb(0 0 0 / 0.3); box-shadow: inset 0 0 0 1px var(--head); }
	.checks li.pass { padding-block: 2px; }
	.checks li.pass .state { color: var(--good); }
	.tag { display: inline-block; margin-left: 6px; padding: 0 8px; border-radius: 999px; font-size: 14px; background: var(--danger); color: var(--ink); vertical-align: 2px; }
	details summary { cursor: pointer; color: var(--muted); }

	.project { display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px; }
	@media (min-width: 700px) { .project:has(.shot) { grid-template-columns: minmax(0, 2fr) minmax(0, 3fr); align-items: start; } }
	.shot { display: block; border-radius: 12px; overflow: hidden; background: var(--deepest); }
	.shot img { display: block; width: 100%; max-height: 280px; object-fit: contain; }
	.about p { margin: 0; }
	.links { margin-top: 6px; }
	.ships { list-style: none; padding: 8px 0 0; margin: 4px 0 0; border-top: 1px solid var(--deep); display: flex; flex-direction: column; gap: 4px; }

	.decide form { gap: 12px; }
	.actions { gap: 4px 16px; }
	.submit { align-self: stretch; }
	.chain { display: flex; flex-direction: column; gap: 6px; padding: 10px 14px; border-radius: 10px; background: rgb(0 0 0 / 0.3); }
	.chain p { margin: 0; }
	.override { padding: 6px 10px; border: 1px solid var(--warn); background: var(--warn-bg); border-radius: 6px; }

	.timeline { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 8px; }
	.timeline li { display: flex; flex-direction: column; gap: 4px; padding: 8px 12px; border-radius: 10px; background: rgb(0 0 0 / 0.25); }
	.timeline li p { margin: 0; }
	.timeline li.internal { background: transparent; border: 2px dashed var(--deep); }
	.timeline li.other { opacity: 0.7; }
	.pre { white-space: pre-wrap; }
	.tiny { font-size: 14px; padding: 0 8px; }
</style>
