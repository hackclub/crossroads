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
	let moving = $state(false);
	// The queue this page belongs to: the ship's stage, or the stage it was
	// just decided at.
	const queueStage = $derived(stage ?? result?.stage ?? 'review');
	// Next review: skip this ship if it's still waiting for you (so the queue
	// doesn't hand it straight back), then claim the next one.
	async function nextReview() {
		moving = true;
		nextMsg = claimError = '';
		const s = queueStage;
		try {
			if (stage && d.can_decide && !result) await api('POST', `/review/submissions/${id}/skip`);
			const res = await api('POST', '/review/next', { stage: s });
			if (res) goto(`/review/${res.submission_id}`);
			else nextMsg = `Nothing left in the ${STAGES[s].label.toLowerCase()} queue for you.`;
		} catch (e) {
			nextMsg = e.message;
		} finally {
			moving = false;
		}
	}

	// Three looks for this page: the site's own, and calmer dark and light
	// ones. Remembered in this browser only.
	const THEMES = [['crossroads', 'Crossroads'], ['dark', 'Dark'], ['light', 'Light']];
	let theme = $state('crossroads');
	onMount(() => {
		try {
			const saved = localStorage.getItem('crossroads.reviewTheme');
			if (THEMES.some(([key]) => key === saved)) theme = saved;
		} catch {}
	});
	function setTheme(key) {
		theme = key;
		try {
			localStorage.setItem('crossroads.reviewTheme', key);
		} catch {}
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
	<div class="review-page" data-theme={theme}>
		<header class="rv-bar"><a class="back" href="/review">Queue</a></header>
		<p class="callout warn" style="margin: 16px">{error}</p>
	</div>
{:else if d}
	{@const p = d.project}
	{@const reship = p.submissions.some((s) => s.id < sub.id && s.status === 'approved')}
	<div class="review-page" data-theme={theme}>
		<!-- The site's nav steps aside on this page: one slim bar to get back to
		     the queue or on to the next ship. -->
		<header class="rv-bar">
			<a class="back" href="/review?stage={queueStage}">
				<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M10 3.5L5.5 8l4.5 4.5" /></svg>
				{STAGES[queueStage].label} queue
			</a>
			<span class="where muted">Ship #{id}</span>
			<span class="claim">
				{#if stage && mine}
					Yours until {time(d.claim.expires_at)}
					<button class="link-btn" onclick={claim}>Renew</button>
				{:else if stage && d.claim}
					{d.claim.name} has it until {time(d.claim.expires_at)}
				{:else if stage && d.can_decide}
					<button class="btn small ghost" onclick={claim}>Claim</button>
				{/if}
			</span>
			<div class="themes" role="group" aria-label="Theme">
				{#each THEMES as [key, label] (key)}
					<button aria-pressed={theme === key} onclick={() => setTheme(key)}>{label}</button>
				{/each}
			</div>
			<button class="btn small primary" onclick={nextReview} disabled={moving} title={stage && d.can_decide && !result ? 'Skips this ship for 3 hours and opens the next one' : ''}>
				{moving ? 'Finding one…' : 'Next review'}
				<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M6 3.5L10.5 8 6 12.5" /></svg>
			</button>
		</header>
		{#if claimError || nextMsg}
			<p class="notice small" role="status">{claimError || nextMsg}</p>
		{/if}

		<div class="body">
			<!-- What a reviewer needs for every ship, always in view. -->
			<section class="base" aria-label="The project">
				{#if p.screenshot_url}
					<a class="shot" href={p.screenshot_url} target="_blank" rel="noreferrer"><img src={p.screenshot_url} alt="Screenshot of {p.name}" /></a>
				{/if}
				<div class="summary">
					<div class="title-row">
						<h1>{p.name}</h1>
						<span class="badge {statusTone(sub.status)}">{STATUS_LABELS[sub.status] ?? sub.status}</span>
						{#if reship}<span class="badge">Update</span>{/if}
					</div>
					<p class="description" title={p.description}>{p.description}</p>
					<div class="links">
						{#if p.demo_url}<a class="btn small primary" href={p.demo_url} target="_blank" rel="noreferrer">Try the demo</a>{/if}
						{#if p.repo_url}<a class="btn small ghost" href={p.repo_url} target="_blank" rel="noreferrer">Repo</a>{/if}
						<span class="muted small">shipped {date(sub.submitted_at)}</span>
					</div>
					<div class="maker small">
						<strong>{d.user.name}</strong>
						<span class="muted">{d.user.slack_id ?? 'no Slack'} · {d.user.verification_status}{d.user.ysws_eligible ? ', YSWS-eligible' : ', not YSWS-eligible'}</span>
						{#if stage === 'fraud' && d.telescreen_url}
							<a href={d.telescreen_url} target="_blank" rel="noreferrer">Telescreen</a>
						{/if}
					</div>
					{#if d.user.reviewer_note}<p class="note small" title={d.user.reviewer_note}>{d.user.reviewer_note}</p>{/if}
				</div>
				<div class="flags">
					<div class="flags-head">
						<h2>Checks</h2>
						<span class="small" class:ok={failing.length === 0}>{failing.length ? `${failing.length} to look at · ${passing.length} passed` : `All ${passing.length} passed`}</span>
					</div>
					{#if failing.length}
						<ul class="checks">
							{#each failing as c (c.id)}
								{@const state = c.status === 'inconclusive' ? 'unknown' : c.severity === 'info' || c.severity === 'warn' ? 'warn' : 'fail'}
								<li class={state} title={c.summary}>
									{@render icon(state)}
									<div>
										<strong>{c.status === 'inconclusive' ? "Couldn't check: " : ''}{c.name}</strong>
										{#if c.fraud}<span class="tag">fraud signal</span>{/if}
										{#if c.summary}<p>{c.summary}</p>{/if}
									</div>
								</li>
							{/each}
						</ul>
					{:else}
						<p class="small muted">Nothing flagged. The full list is below.</p>
					{/if}
				</div>
			</section>

			<!-- The details, for when the summary isn't enough. Only this part scrolls. -->
			<div class="evidence">
				<section class="part">
					<CodeActivity github={d.github} githubError={d.github_error} days={d.days} windowStart={d.window_start} repoUrl={p.repo_url} {reship} />
				</section>

				<section class="part">
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

				<section class="part two">
					<div class="stack-sm">
						<h2>Every check</h2>
						<ul class="checks">
							{#each d.checks as c (c.id)}
								{@const state = c.status === 'pass' ? 'pass' : c.status === 'inconclusive' ? 'unknown' : c.severity === 'info' || c.severity === 'warn' ? 'warn' : 'fail'}
								<li class="{state} flat">{@render icon(state)}<div><span>{c.name}</span>{#if c.summary}<p>{c.summary}</p>{/if}</div></li>
							{/each}
						</ul>
					</div>
					<div class="stack-sm">
						<h2>{d.user.name}</h2>
						<p class="small muted">{d.user.email}</p>
						{#if noteDraft === null}
							{#if d.user.reviewer_note}<p class="pre small note full">{d.user.reviewer_note}</p>{/if}
							<button class="link-btn small" onclick={() => (noteDraft = d.user.reviewer_note ?? '')}>{d.user.reviewer_note ? 'Edit the note on them' : 'Add a note on them'}</button>
						{:else}
							<label class="field">
								<span>Note on {d.user.name}</span>
								<textarea bind:value={noteDraft} style="min-height: 80px"></textarea>
								<small>Reviewers only. Shown on all their ships.</small>
							</label>
							<div class="row">
								<button class="btn small primary" onclick={saveNote}>Save</button>
								<button class="btn small ghost" onclick={() => (noteDraft = null)}>Cancel</button>
							</div>
							{#if noteError}<p class="callout warn small">{noteError}</p>{/if}
						{/if}
						{#if p.submissions.length > 1}
							<h2 style="margin-top: 12px">Ships</h2>
							<ul class="ships small">
								{#each p.submissions as s (s.id)}
									<li>
										{#if s.id === sub.id}<strong>This ship</strong>{:else}<a href="/review/{s.id}">#{s.id}</a>{/if}
										<span class="muted">{STATUS_LABELS[s.status] ?? s.status} · {hours(s.new_seconds)} claimed{s.approved_seconds != null ? `, ${hours(s.approved_seconds)} approved` : ''} · {date(s.submitted_at)}</span>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				</section>
			</div>

			<!-- The hours and the decision, sized to the window. -->
			<aside class="rail" id="decide" aria-label="Decision">
				<dl class="numbers">
					<div class="lead"><dt>Claimed</dt><dd>{hours(sub.new_seconds)}</dd></div>
					{#if sub.approved_seconds != null}<div><dt>Approved so far</dt><dd>{hours(sub.approved_seconds)}</dd></div>{/if}
					<div><dt>Hackatime now</dt><dd>{d.hackatime ? hours(d.hackatime.seconds) : '–'}</dd></div>
					<div><dt>Earlier ships</dt><dd>{hours(p.approved_seconds)}</dd></div>
				</dl>

				{#if result}
					<div class="callout good small" role="status">
						<strong>Saved. It's now: {STATUS_LABELS[d.submission.status] ?? d.submission.status}.</strong>
						{#if result.stamps_paid || result.boxes_granted.length}
							<span>Paid {result.stamps_paid} stamps{result.boxes_granted.length ? ` and gave ${result.boxes_granted.length} box(es)` : ''}.</span>
						{/if}
					</div>
				{/if}
				{#if stage && d.can_decide}
					<form class="decision" onsubmit={decide}>
						<h2>{STAGES[stage].label}</h2>
						<div class="form-cols">
							<div class="form-a">
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
							</div>
							<div class="form-b">
						{#if action !== 'send_back'}
							<label class="field grow">
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
						<label class="field grow">
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
							</div>
						</div>
					</form>
				{:else if stage}
					<h2>{STAGES[stage].label}</h2>
					<p class="callout info small">{d.cant_decide_reason}</p>
				{:else if !result}
					<p class="callout info small">This ship has been decided: {STATUS_LABELS[sub.status] ?? sub.status}{sub.approved_seconds != null ? `, ${hours(sub.approved_seconds)}` : ''}.</p>
				{/if}
			</aside>
		</div>
	</div>
{:else}
	<p class="loading muted">Loading… (this asks Hackatime and GitHub, so it can take a moment)</p>
{/if}

<style>
	/* The review page fills the window like an app (after Ari): a slim bar,
	   then the summary across the top, the details under it (the only part
	   that scrolls), and the decision in a rail sized to the window. On small
	   or short screens it falls back to one scrolling page. */
	.review-page {
		/* Tokens of its own, so the three themes below can swap them. */
		--rv-page: transparent;
		--rv-surface: var(--row-bg);
		--rv-rail: rgb(0 0 0 / 0.3);
		--rv-line: rgb(255 255 255 / 0.13);
		--rv-sunk: rgb(0 0 0 / 0.28);
		--rv-bar: rgb(0 0 0 / 0.55);
		--rv-on-danger: #000;
		font-family: 'Crimson Pro', Georgia, serif;
		font-size: 19px;
		color: var(--text);
		background: var(--rv-page);
		min-height: 100dvh;
		display: flex;
		flex-direction: column;
	}
	/* Dark and light are calmer than the photo: a soft slate and a warm paper,
	   neither pure black nor white. */
	.review-page[data-theme='dark'] {
		color-scheme: dark;
		--bg: #1f2227; --rv-page: #1f2227; --rv-surface: #262a30; --rv-rail: #22262b; --rv-bar: #1b1e22;
		--rv-line: #363c44; --rv-sunk: #1d2025; --row-bg: #262a30;
		--text: #e4e2dc; --muted: #a8acb2; --faint: #9095a0; --ink: #16181b;
		--body: #3a4049; --head: #5d6776; --deep: #353b43; --deepest: #191b1f; --light: #4a515b;
		--navy: #2f3540; --navy-deep: #1b1e23;
		--good: #93c77f; --good-bg: #26312a; --warn: #e2a56e; --warn-bg: #3a2f26;
		--info: #8fb3e3; --info-bg: #243142; --danger: #e8836f; --rv-on-danger: #16181b;
	}
	.review-page[data-theme='light'] {
		color-scheme: light;
		--bg: #e8e4dc; --rv-page: #e8e4dc; --rv-surface: #f3f0ea; --rv-rail: #ece8e0; --rv-bar: #ddd8ce;
		--rv-line: #d3cdc2; --rv-sunk: #e6e1d8; --row-bg: #f3f0ea;
		--text: #2b2c2f; --muted: #5d6067; --faint: #6c6f76; --ink: #2b2c2f;
		--body: #dbd5ca; --head: #c3baa9; --deep: #ddd7cc; --deepest: #e4dfd6; --light: #cfc8bb;
		--navy: #e0dad0; --navy-deep: #fbf9f5;
		--good: #3f7a2c; --good-bg: #e0ead6; --warn: #a3541d; --warn-bg: #f2e3d5;
		--info: #2f5f9c; --info-bg: #dce5f0; --danger: #b13b27; --rv-on-danger: #fff;
	}
	.review-page[data-theme='light'] :global(.callout),
	.review-page[data-theme='dark'] :global(.callout) { border-color: var(--rv-line); }
	.review-page[data-theme='light'] :global(.btn.ghost),
	.review-page[data-theme='dark'] :global(.btn.ghost) { background: transparent; }
	.review-page :global(.small) { font-size: 17px; }
	.review-page :global(h2) { font-size: 24px; }
	.review-page :global(.btn.small) { font-size: 18px; padding: 2px 12px; }
	.review-page :global(.badge) { font-size: 16px; padding: 1px 10px; }
	.review-page :global(.field > span) { font-size: 18px; padding: 3px 12px; border-radius: 10px 10px 0 0; }
	.review-page :global(.field input), .review-page :global(.field textarea), .review-page :global(.field select) { font-size: 18px; padding: 6px 12px; border-radius: 0 0 10px 10px; }
	.review-page :global(.field small) { font-size: 15px; padding-top: 3px; }
	.review-page :global(.callout) { margin: 0; padding: 10px 14px; }
	.loading { padding: 24px; }

	.rv-bar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px 14px;
		padding: 8px 16px;
		background: var(--rv-bar);
		border-bottom: 1px solid var(--rv-line);
		font-size: 17px;
	}
	.back { display: inline-flex; align-items: center; gap: 2px; padding: 2px 12px 2px 6px; border-radius: 999px; box-shadow: inset 0 0 0 1px var(--head); color: var(--text); text-decoration: none; }
	.back:hover { background: var(--deep); }
	.back svg, .rv-bar .btn svg, .state { width: 16px; height: 16px; flex: none; fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
	.claim { margin-left: auto; display: inline-flex; align-items: center; gap: 8px; color: var(--muted); }
	.themes { display: inline-flex; padding: 2px; border-radius: 999px; background: var(--rv-sunk); }
	.themes button { padding: 1px 12px; border: 0; border-radius: 999px; background: transparent; color: var(--muted); font: inherit; font-size: 16px; cursor: pointer; }
	.themes button[aria-pressed='true'] { background: var(--body); color: var(--text); }
	.notice { margin: 0; padding: 6px 16px; background: var(--info-bg); border-bottom: 1px solid var(--rv-line); }

	.body {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		grid-template-areas: 'base' 'rail' 'evidence';
		background: var(--rv-surface);
		flex: 1;
	}
	.base { grid-area: base; }
	.evidence { grid-area: evidence; }
	.rail { grid-area: rail; }

	.base { display: grid; grid-template-columns: minmax(0, 1fr); gap: 14px 22px; padding: 16px 20px; border-bottom: 1px solid var(--rv-line); }
	@media (min-width: 760px) {
		.base { grid-template-columns: minmax(0, 200px) minmax(0, 1.15fr) minmax(0, 1fr); }
		.base:not(:has(.shot)) { grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); }
	}
	.shot { display: block; align-self: start; border-radius: 10px; overflow: hidden; background: var(--deepest); }
	.shot img { display: block; width: 100%; max-height: 180px; object-fit: contain; }
	.summary { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
	.title-row { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 10px; }
	.title-row h1 { font-size: 34px; line-height: 1.05; overflow-wrap: anywhere; }
	.description { margin: 0; display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
	.links { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
	.maker { display: flex; flex-wrap: wrap; align-items: baseline; gap: 2px 10px; }
	.note { margin: 0; padding: 4px 10px; border-radius: 8px; background: var(--rv-sunk); display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
	.note.full { display: block; white-space: pre-wrap; }
	.flags { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
	.flags-head { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; }

	.checks { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 5px; }
	.checks li { display: flex; gap: 8px; align-items: flex-start; padding: 5px 10px; border-radius: 8px; font-size: 17px; line-height: 1.3; }
	.checks li .state { margin-top: 2px; }
	.checks li p { margin: 1px 0 0; color: var(--muted); font-size: 16px; }
	.flags .checks li p { display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
	.checks li.fail { background: var(--warn-bg); box-shadow: inset 0 0 0 1px var(--danger); }
	.checks li.fail .state { color: var(--danger); }
	.checks li.warn { background: var(--warn-bg); }
	.checks li.warn .state { color: var(--warn); }
	.checks li.unknown { background: var(--rv-sunk); box-shadow: inset 0 0 0 1px var(--head); }
	.checks li.pass .state { color: var(--good); }
	.checks li.flat { padding-block: 3px; }
	.tag { display: inline-block; margin-left: 6px; padding: 0 8px; border-radius: 999px; font-size: 13px; background: var(--danger); color: var(--rv-on-danger); vertical-align: 2px; }

	.evidence { display: flex; flex-direction: column; min-width: 0; }
	.part { display: flex; flex-direction: column; gap: 12px; padding: 16px 20px; min-width: 0; }
	.part + .part { border-top: 1px solid var(--rv-line); }
	.two { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px 28px; }
	@media (min-width: 1200px) { .two { grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr); } }
	.ships { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 2px; }

	.rail { display: flex; flex-direction: column; gap: 12px; padding: 14px 18px; background: var(--rv-rail); border-bottom: 1px solid var(--rv-line); min-width: 0; }
	.numbers { display: grid; grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); gap: 6px; margin: 0; }
	.numbers div { padding: 4px 10px; border-radius: 8px; background: var(--rv-sunk); }
	.numbers dt { color: var(--muted); font-size: 14px; }
	.numbers dd { margin: 0; font-size: 20px; font-variant-numeric: tabular-nums; }
	.numbers .lead dd { font-size: 24px; }

	.decision { display: flex; flex-direction: column; gap: 10px; flex: 1; min-height: 0; }
	.form-cols, .form-a, .form-b { display: flex; flex-direction: column; gap: 10px; min-width: 0; }
	.actions { gap: 4px 16px; }
	.submit { align-self: stretch; }
	.chain { display: flex; flex-direction: column; gap: 4px; padding: 8px 12px; border-radius: 10px; background: var(--rv-sunk); max-height: 150px; overflow-y: auto; scrollbar-width: thin; }
	.chain p { margin: 0; }
	.override { padding: 4px 8px; border: 1px solid var(--warn); background: var(--warn-bg); border-radius: 6px; }

	.timeline { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 8px; }
	.timeline li { display: flex; flex-direction: column; gap: 4px; padding: 8px 12px; border-radius: 10px; background: var(--rv-sunk); }
	.timeline li p { margin: 0; }
	.timeline li.internal { background: transparent; border: 2px dashed var(--deep); }
	.timeline li.other { opacity: 0.7; }
	.pre { white-space: pre-wrap; }
	.tiny { font-size: 14px; padding: 0 8px; }

	/* Big enough for the app layout: nothing but the details scrolls, and the
	   rail's text boxes share whatever height is left. */
	@media (min-width: 1000px) and (min-height: 620px) {
		.review-page { height: 100dvh; overflow: hidden; }
		.body {
			min-height: 0;
			grid-template-columns: minmax(0, 1fr) minmax(0, 420px);
			grid-template-rows: auto minmax(0, 1fr);
			grid-template-areas: 'base rail' 'evidence rail';
		}
		.base { max-height: 42dvh; overflow-y: auto; scrollbar-width: thin; }
		.evidence { overflow-y: auto; min-height: 0; scrollbar-width: thin; }
		.rail { border-bottom: 0; border-left: 1px solid var(--rv-line); min-height: 0; overflow-y: auto; scrollbar-width: none; }
		.decision .form-b { flex: 1; min-height: 0; }
		.decision .form-cols { flex: 1; min-height: 0; }
		.decision .grow { flex: 1 1 0; min-height: 0; }
		.decision .grow textarea { flex: 1; min-height: 44px; resize: none; }
	}
</style>

