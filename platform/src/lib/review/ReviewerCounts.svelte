<script>
	// Each reviewer's reviews in a period, and the reviews behind a row when
	// it's opened: the admin page's Admin review stats and the review page's
	// Review stats. base is '/admin' or '/review'; reviewers can only open
	// their own row (GET /review/reviews refuses anyone else's).
	import { get } from '../api.js';
	import { hours } from '../format.js';
	import { ZONE } from './period.js';

	let { reviewers, query, base, canOpen = () => true } = $props();

	let open = $state(null); // the reviewer whose reviews are listed
	let log = $state(null);
	let logError = $state('');

	// A new period closes the open row.
	$effect(() => {
		query;
		open = log = null;
	});

	async function toggle(r) {
		if (open === r.id) {
			open = log = null;
			return;
		}
		open = r.id;
		log = null;
		logError = '';
		try {
			log = await get(`${base}/reviews?${query}&reviewer=${r.id}`);
		} catch (e) {
			logError = e.message;
		}
	}

	const STAGE = { pending: 'First review', fraud_check: 'Fraud check', awaiting_hq: 'HQ sign-off' };
	const DECISION = { approval: 'Approved', rejection: 'Rejected', fraud_pass: 'Passed', final_approval: 'Signed off', sent_back: 'Sent back' };
	const when = (iso) => new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', timeZone: ZONE });
</script>

{#if reviewers.length}
	<table>
		<thead>
			<tr>
				<th>Reviewer</th>
				<th class="num">Reviews</th>
				<th class="num" title="Approved / rejected">First reviews</th>
				<th class="num">Fraud checks</th>
				<th class="num">HQ</th>
				<th class="num" title="First reviews whose approval was later sent back">Sent back later</th>
				<th class="num" title="Hours the ships claimed, for their first reviews">Hours reviewed</th>
				<th>Last review</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{#each reviewers as r (r.id)}
				<tr class:open={open === r.id}>
					<td>
						<div class="who">
							<span>{r.name}</span>
							{#if r.email}<span class="muted tiny">{r.slack_id ?? 'no Slack'} · {r.email}</span>{/if}
						</div>
					</td>
					<td class="num total">{r.reviews}</td>
					<td class="num">{r.first_reviews} <span class="muted tiny">{r.first_approvals} / {r.first_rejections}</span></td>
					<td class="num">{r.fraud_checks}</td>
					<td class="num">{r.hq_decisions}</td>
					<td class="num">{r.later_sent_back || '–'}</td>
					<td class="num">{hours(r.seconds_first_reviewed)}</td>
					<td class="small">{when(r.last_review_at)}</td>
					<td>
						{#if canOpen(r)}
							<button class="btn small ghost" aria-expanded={open === r.id} onclick={() => toggle(r)}>{open === r.id ? 'Hide' : 'Reviews'}</button>
						{/if}
					</td>
				</tr>
				{#if open === r.id}
					<tr class="log-row">
						<td colspan="9">
							{#if logError}
								<p class="callout warn small">{logError}</p>
							{:else if !log}
								<p class="muted small">Loading…</p>
							{:else}
								<table class="log">
									<thead><tr><th>When (Eastern)</th><th>Stage</th><th>Decision</th><th>Ship</th><th>Project</th><th class="num">Claimed</th><th class="num">Approved</th><th></th></tr></thead>
									<tbody>
										{#each log as e (e.id)}
											<tr>
												<td>{when(e.created_at)}</td>
												<td>{STAGE[e.stage] ?? e.stage}</td>
												<td>{DECISION[e.kind] ?? e.kind}</td>
												<td><a href="/review/{e.submission_id}">#{e.submission_id}</a></td>
												<td>{e.project_name} <span class="muted">by {e.maker_name}</span></td>
												<td class="num">{hours(e.new_seconds)}</td>
												<td class="num">{e.seconds == null ? '–' : hours(e.seconds)}</td>
												<td>{#if e.later_sent_back}<span class="badge warn">Sent back later</span>{/if}</td>
											</tr>
										{/each}
									</tbody>
								</table>
							{/if}
						</td>
					</tr>
				{/if}
			{/each}
		</tbody>
	</table>
{:else}
	<p class="muted empty">No reviews in this period.</p>
{/if}

<style>
	.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
	.total { font-size: 28px; }
	.who { display: flex; flex-direction: column; }
	.tiny { font-size: 17px; }
	tr.open > td { background: var(--deep); }
	.log-row > td { padding: 0 16px 16px; background: var(--deep); }
	.log { font-size: 19px; background: var(--row-bg); border-radius: 10px; overflow: hidden; }
	.log th { font-size: 19px; padding: 6px 12px; }
	.log td { padding: 6px 12px; }
	.empty { padding: 0 16px 16px; }
</style>
