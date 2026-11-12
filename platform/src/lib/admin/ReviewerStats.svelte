<script>
	// How many reviews each person made in a period, for paying reviewers
	// (GET /admin/reviewer-stats and /admin/reviews). A review is one decision
	// on a ship; see backend/db/queries/reviewstats.sql. Days run midnight to
	// midnight Eastern, and a finished period's counts never change.
	import { onMount } from 'svelte';
	import { get } from '../api.js';
	import { hours } from '../format.js';

	const ZONE = 'America/New_York';
	// Today in New York as YYYY-MM-DD, and date arithmetic on such strings.
	const today = () => new Date().toLocaleDateString('en-CA', { timeZone: ZONE });
	const shift = (day, days) => {
		const d = new Date(day + 'T12:00:00Z');
		d.setUTCDate(d.getUTCDate() + days);
		return d.toISOString().slice(0, 10);
	};
	const monday = (day) => shift(day, -((new Date(day + 'T12:00:00Z').getUTCDay() + 6) % 7));
	const monthStart = (day) => day.slice(0, 8) + '01';

	const PRESETS = [
		['week', 'This week', () => [monday(today()), today()]],
		['last-week', 'Last week', () => [shift(monday(today()), -7), shift(monday(today()), -1)]],
		['month', 'This month', () => [monthStart(today()), today()]],
		['last-month', 'Last month', () => {
			const end = shift(monthStart(today()), -1);
			return [monthStart(end), end];
		}],
		['all', 'All time', () => ['2026-10-30', today()]]
	];

	let preset = $state('week');
	let from = $state(monday(today()));
	let to = $state(today());
	let data = $state(null);
	let loading = $state(false);
	let error = $state('');
	let open = $state(null); // the reviewer whose reviews are listed
	let log = $state(null);
	let logError = $state('');

	const query = $derived(`from=${from}&to=${to}`);

	async function load() {
		loading = true;
		error = '';
		open = log = null;
		try {
			data = await get(`/admin/reviewer-stats?${query}`);
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}
	onMount(load);

	function choose(key) {
		preset = key;
		[from, to] = PRESETS.find(([k]) => k === key)[2]();
		load();
	}
	// A start after the end moves the end to it, and the other way round.
	function custom(changed) {
		preset = null;
		if (!from || !to) return;
		if (from > to) {
			if (changed === 'from') to = from;
			else from = to;
		}
		load();
	}

	async function toggle(r) {
		if (open === r.id) {
			open = log = null;
			return;
		}
		open = r.id;
		log = null;
		logError = '';
		try {
			log = await get(`/admin/reviews?${query}&reviewer=${r.id}`);
		} catch (e) {
			logError = e.message;
		}
	}

	const STAGE = { pending: 'First review', fraud_check: 'Fraud check', awaiting_hq: 'HQ sign-off' };
	const DECISION = { approval: 'Approved', rejection: 'Rejected', fraud_pass: 'Passed', final_approval: 'Signed off', sent_back: 'Sent back' };
	const when = (iso) => new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', timeZone: ZONE });
	const dayLabel = (day) => new Date(day + 'T12:00:00Z').toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric', timeZone: 'UTC' });
</script>

<div class="stack">
	<div class="row filters">
		<div class="presets" role="group" aria-label="Period">
			{#each PRESETS as [key, label]}
				<button class="btn small" class:primary={preset === key} aria-pressed={preset === key} onclick={() => choose(key)}>{label}</button>
			{/each}
		</div>
		<div class="range">
			<label>From <input type="date" bind:value={from} onchange={() => custom('from')} /></label>
			<label>to <input type="date" bind:value={to} onchange={() => custom('to')} /></label>
		</div>
	</div>

	<p class="small muted">
		A review is one decision on a ship: approving or rejecting it at the first review, passing or rejecting it at the fraud
		check, signing off or rejecting at HQ, or sending it back. Comments, notes and edits don't count, and Sidekick decisions
		count the same. Days run midnight to midnight Eastern time, and once a period is over its counts never change.
	</p>

	{#if error}<p class="callout warn">{error}</p>{/if}

	{#if data}
		<div class="card table-wrap" class:loading>
			<div class="summary">
				<div>
					<strong>{data.reviews.toLocaleString()}</strong> review{data.reviews === 1 ? '' : 's'} by
					<strong>{data.reviewers.length}</strong> {data.reviewers.length === 1 ? 'person' : 'people'},
					{dayLabel(data.from)}{data.from === data.to ? '' : ` to ${dayLabel(data.to)}`}
				</div>
				<div class="downloads">
					<a class="btn small ghost" href="/api/admin/reviewer-stats?{query}&format=csv" download>Counts (CSV)</a>
					<a class="btn small ghost" href="/api/admin/reviews?{query}&format=csv" download>Every review (CSV)</a>
				</div>
			</div>
			{#if data.reviewers.length}
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
						{#each data.reviewers as r (r.id)}
							<tr class:open={open === r.id}>
								<td>
									<div class="who">
										<span>{r.name}</span>
										<span class="muted tiny">{r.slack_id ?? 'no Slack'} · {r.email}</span>
									</div>
								</td>
								<td class="num total">{r.reviews}</td>
								<td class="num">{r.first_reviews} <span class="muted tiny">{r.first_approvals} / {r.first_rejections}</span></td>
								<td class="num">{r.fraud_checks}</td>
								<td class="num">{r.hq_decisions}</td>
								<td class="num">{r.later_sent_back || '–'}</td>
								<td class="num">{hours(r.seconds_first_reviewed)}</td>
								<td class="small">{when(r.last_review_at)}</td>
								<td><button class="btn small ghost" aria-expanded={open === r.id} onclick={() => toggle(r)}>{open === r.id ? 'Hide' : 'Reviews'}</button></td>
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
		</div>
	{/if}
</div>

<style>
	.filters { justify-content: space-between; align-items: center; gap: 12px; }
	.presets { display: flex; flex-wrap: wrap; gap: 6px; }
	.range { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; font-size: 22px; }
	.range label { display: inline-flex; align-items: center; gap: 8px; }
	.range input { width: auto; padding: 4px 10px; font-size: 20px; }
	.loading { opacity: 0.55; transition: opacity 0.2s; }

	.summary { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; padding: 14px 16px; }
	.downloads { display: flex; flex-wrap: wrap; gap: 8px; }
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
