<script>
	// The three review queues: ships waiting for a reviewer, for the fraud
	// check, and for HQ sign-off. See backend/api/stages.go for the rules.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, get } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import { hours, date } from '#lib/format.js';
	import { STAGES, canDecideStage, reasonLabel, time } from '#lib/review.js';

	const stage = $derived(STAGES[page.url.searchParams.get('stage')] ? page.url.searchParams.get('stage') : 'review');
	const canDecide = $derived(canDecideStage(session.me, stage));

	let data = $state(null);
	let error = $state('');
	let nextMsg = $state('');
	let busy = $state(false);

	async function load(s) {
		error = '';
		try {
			data = await get(`/review/queue?stage=${s}`);
		} catch (e) {
			error = e.message;
		}
	}
	$effect(() => {
		nextMsg = '';
		load(stage);
	});

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
		<p class="muted">{STAGES[stage].blurb}</p>
	</div>
	<div class="row">
		<a class="btn small ghost" href="/review/templates">Feedback templates</a>
		{#if canDecide}
			<button class="btn primary" disabled={busy} onclick={next}>{busy ? 'Finding one…' : 'Next'}</button>
		{/if}
	</div>
</div>

<div class="tabs" role="tablist">
	{#each Object.entries(STAGES) as [key, s]}
		<button role="tab" aria-selected={stage === key} class:active={stage === key} onclick={() => goto(`?stage=${key}`, { replaceState: true, noScroll: true })}>
			{s.label}{#if data?.counts} ({data.counts[key]}){/if}
		</button>
	{/each}
</div>

<div class="stack">
	{#if !canDecide}
		<p class="callout info small">You can look, but you can't decide ships at {STAGES[stage].label.toLowerCase()}. An admin can give you that on the Admin page.</p>
	{/if}
	{#if nextMsg}<p class="callout info small" role="status">{nextMsg}</p>{/if}
	{#if error}<p class="callout warn">{error}</p>{/if}

	{#if data?.items.length}
		<div class="card table-wrap">
			<table>
				<thead>
					<tr>
						<th>Project</th><th>By</th><th>Claims</th>
						{#if stage !== 'review'}<th>Approved</th><th>Reviewer</th>{/if}
						<th>Shipped</th><th>Who's on it</th><th></th>
					</tr>
				</thead>
				<tbody>
					{#each data.items as q}
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
							<td>{hours(q.new_seconds)}</td>
							{#if stage !== 'review'}
								<td>{hours(q.approved_seconds)}</td>
								<td class="small">{q.reviewer_name ?? '–'}</td>
							{/if}
							<td class="small">{date(q.submitted_at)}</td>
							<td class="small">
								{#if q.claim}
									{q.claim.user_id === session.me.id ? 'You' : q.claim.name}, until {time(q.claim.expires_at)}
								{:else}<span class="muted">Nobody</span>{/if}
								{#if q.skipped_by_me}<br /><span class="muted">You skipped it</span>{/if}
							</td>
							<td><a class="btn small {q.mine ? 'ghost' : 'primary'}" href="/review/{q.id}">{q.mine ? 'View' : 'Open'}</a></td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else if data}
		<div class="card dashed"><p class="muted">Nothing waiting here.</p></div>
	{/if}
</div>

<style>
	.tiny { font-size: 17px; padding: 0 8px; }
</style>
