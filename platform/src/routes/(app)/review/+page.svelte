<script>
	// The three review queues: ships waiting for a reviewer, for the fraud
	// check, and for HQ sign-off. See backend/api/stages.go for the rules.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, get, peek } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import { hours, date } from '#lib/format.js';
	import { STAGES, canDecideStage, canManageRejections, canPrereview, reasonLabel, time } from '#lib/review.js';
	import ReviewDashboard from '#lib/review/ReviewDashboard.svelte';
	import PastReviews from '#lib/review/PastReviews.svelte';
	import Prereviews from '#lib/review/Prereviews.svelte';
	import ReviewStats from '#lib/review/ReviewStats.svelte';

	// Besides the three queues: decided ships, (HQ and admins) every
	// rejection, pre-reviews, and the review stats.
	const EXTRA = $derived({
		past: { label: 'Past reviews', blurb: 'Ships that have been decided. Open one to see what happened.' },
		...(canManageRejections(session.me) ? { rejected: { label: 'Rejected', blurb: 'Every rejected ship. Change a soft reject to hard or back, or undo a rejection.' } } : {}),
		...(canPrereview(session.me) ? { prereviews: { label: 'Pre-reviews', blurb: 'Have your own Claude pre-review the ships waiting, with a prompt to copy and an answer to paste back.' } } : {}),
		stats: { label: 'Review stats', blurb: "How the queues are doing: what's waiting, how many get rejected, where ships end up, and everyone's reviews." }
	});
	const wanted = $derived(page.url.searchParams.get('stage'));
	const stage = $derived(STAGES[wanted] || EXTRA[wanted] ? wanted : 'review');
	const queue = $derived(!!STAGES[stage]);
	const canDecide = $derived(queue && canDecideStage(session.me, stage));

	let data = $state(null);
	let error = $state('');
	let nextMsg = $state('');
	let busy = $state(false);

	async function load(s) {
		error = '';
		// The queue as it was last seen, while it's asked for again.
		data = peek(`/review/queue?stage=${s}`) ?? null;
		try {
			const got = await get(`/review/queue?stage=${s}`);
			if (s === stage) data = got; // not if they've switched queue since
		} catch (e) {
			if (s === stage) error = e.message;
		}
	}
	$effect(() => {
		nextMsg = '';
		if (queue) load(stage);
	});

	// Ships the fraud check or HQ sent back to the reviewer who approved them,
	// to change their review: in a list of their own above the queue, the
	// viewer's own first. Only that reviewer gets them from Next, but anyone
	// can open one, so nothing's stuck if they're away.
	const returned = $derived((data?.items ?? []).filter((q) => q.returned).toSorted((a, b) => b.returned.to_me - a.returned.to_me));
	const waiting = $derived((data?.items ?? []).filter((q) => !q.returned));
	const FROM = { fraud: 'the fraud check', hq: 'HQ sign-off' };
	const returnedToMe = $derived(returned.filter((q) => q.returned.to_me).length);

	async function next() {
		busy = true;
		nextMsg = '';
		try {
			const res = await api('POST', '/review/next', { stage });
			if (res) goto(`/review/${res.submission_id}`);
			else nextMsg = "Nothing left for you here: everything's taken, skipped, or yours.";
		} catch (e) {
			nextMsg = e.message;
		} finally {
			busy = false;
		}
	}
</script>

<div class="page-head">
	<div class="stack-sm">
		<h1>Review</h1>
		<p class="muted">{(STAGES[stage] ?? EXTRA[stage]).blurb}</p>
	</div>
	<div class="row">
		<a class="btn small ghost" href="/review/templates">Feedback templates</a>
		{#if canDecide}
			<button class="btn primary" disabled={busy} onclick={next}>{busy ? 'Finding one…' : 'Next'}</button>
		{/if}
	</div>
</div>

<ReviewDashboard />

<div class="tabs" role="tablist">
	{#each Object.entries(STAGES) as [key, s]}
		<button role="tab" aria-selected={stage === key} class:active={stage === key} onclick={() => goto(`?stage=${key}`, { replace: true, reset: false })}>
			{s.label}{#if data?.counts}{' '}<span class="tab-count">{data.counts[key]}</span>{/if}
		</button>
	{/each}
	{#each Object.entries(EXTRA) as [key, s]}
		<button role="tab" aria-selected={stage === key} class:active={stage === key} onclick={() => goto(`?stage=${key}`, { replace: true, reset: false })}>{s.label}</button>
	{/each}
</div>

{#if stage === 'prereviews'}
	<Prereviews />
{:else if stage === 'stats'}
	<ReviewStats />
{:else if !queue}
	{#key stage}<PastReviews mode={stage} />{/key}
{:else}
<div class="stack">
	{#if !canDecide}
		<p class="callout info small">You can look, but you can't decide ships at {STAGES[stage].label.toLowerCase()}. An admin can give you that on the Admin page.</p>
	{/if}
	{#if nextMsg}<p class="callout info small" role="status">{nextMsg}</p>{/if}
	{#if error}<p class="callout warn">{error}</p>{/if}

	{#if returned.length}
		<section class="card returned" aria-labelledby="returned-head">
			<div class="returned-head">
				<h2 id="returned-head">Changes requested</h2>
				<span class="small muted">
					{returnedToMe ? `${returnedToMe} for you. ` : ''}Sent back by the fraud check or HQ to the reviewer who approved them. Next gives you yours first.
				</span>
			</div>
			<ul>
				{#each returned as q (q.id)}
					<li class:mine={q.returned.to_me}>
						<div class="what">
							<span>{q.project_name} <span class="muted small">by {q.user_name}</span></span>
							<span class="small">
								<span class="muted">{q.returned.by || 'Someone'} at {FROM[q.returned.from] ?? 'a later stage'}, for {q.returned.to_me ? 'you' : q.returned.to.name}:</span>
								{q.returned.note}
							</span>
						</div>
						<span class="small muted claim">
							{#if q.claim}{q.claim.user_id === session.me.id ? 'You' : q.claim.name}, until {time(q.claim.expires_at)}{/if}
						</span>
						<a class="btn small {q.returned.to_me ? 'primary' : 'ghost'}" href="/review/{q.id}">{q.returned.to_me ? 'Change it' : 'Open'}</a>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	{#if waiting.length}
		<div class="card table-wrap">
			<table>
				<thead>
					<tr>
						<th>Project</th><th>By</th><th class="num">Claims</th>
						{#if stage !== 'review'}<th class="num">Approved</th><th>Reviewer</th>{/if}
						<th>Shipped</th><th>Who's on it</th><th></th>
					</tr>
				</thead>
				<tbody>
					{#each waiting as q}
						<tr>
							<td>
								<div class="stack-sm">
									<span>{#if q.priority}<span class="badge warn">Priority</span> {/if}{q.project_name}</span>
									{#if q.fraud_reasons?.length || q.override_reason}
										<span class="pill-row">
											{#each q.fraud_reasons as r}<span class="badge warn tiny">{reasonLabel(r)}</span>{/each}
											{#if q.override_reason}<span class="badge info tiny" title={q.override_reason}>Over Hackatime</span>{/if}
										</span>
									{/if}
								</div>
							</td>
							<td>{q.user_name}{#if q.mine} <span class="badge">You</span>{/if}</td>
							<td class="num">{hours(q.new_seconds)}</td>
							{#if stage !== 'review'}
								<td class="num">{hours(q.approved_seconds)}</td>
								<td class="small">{q.reviewer_name ?? '–'}</td>
							{/if}
							<td class="small">{date(q.submitted_at)}</td>
							<td class="small">
								{#if q.claim}
									{q.claim.user_id === session.me.id ? 'You' : q.claim.name}, until {time(q.claim.expires_at)}
								{:else}<span class="muted" aria-label="Nobody">–</span>{/if}
								{#if q.skipped_by_me}<br /><span class="muted">You skipped it</span>{/if}
							</td>
							<td><a class="btn small {q.mine ? 'ghost' : 'primary'}" href="/review/{q.id}">{q.mine ? 'View' : 'Open'}</a></td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else if data && !returned.length}
		<div class="card dashed"><p class="muted">Nothing waiting here.</p></div>
	{/if}
</div>
{/if}

<style>
	.tiny { font-size: 17px; padding: 0 8px; }
	.num { text-align: right; font-variant-numeric: tabular-nums; }
	/* Changes requested: a short list above the queue, the viewer's own
	   marked with the info edge. */
	.returned { padding: 0; overflow: hidden; }
	.returned-head { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 4px 12px; padding: 12px 16px; }
	.returned h2 { font-size: 24px; margin: 0; }
	.returned ul { list-style: none; margin: 0; padding: 0; }
	.returned li { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 6px 14px; padding: 10px 16px; border-top: 1px solid var(--deep); }
	.returned li.mine { box-shadow: inset 4px 0 0 var(--info); }
	.returned .what { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.returned .what .small { overflow-wrap: anywhere; white-space: pre-line; display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
	@media (max-width: 560px) {
		.returned li { grid-template-columns: minmax(0, 1fr) auto; }
		.returned .claim { grid-column: 1 / -1; grid-row: 2; }
	}
	.tab-count { display: inline-block; min-width: 1.6em; padding: 0 8px; border-radius: 999px; background: rgb(0 0 0 / 0.3); font-size: 0.8em; text-align: center; font-variant-numeric: tabular-nums; }
</style>
