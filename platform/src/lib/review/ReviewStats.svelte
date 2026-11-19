<script>
	// The review page's Review stats tab: more than the dashboard above it,
	// for any period. What's waiting at each stage now and the oldest ship,
	// the period's reviews and rejection rates, where the ships shipped in it
	// got to, and everyone's reviews (GET /review/insights and
	// /review/reviewer-stats, backend/api/reviewstats.go). Reviewers can open
	// their own reviews; admins anyone's.
	import { onMount } from 'svelte';
	import { get } from '#lib/api.js';
	import { session, isAdmin } from '#lib/session.svelte.js';
	import { hours } from '#lib/format.js';
	import { STAGES } from '#lib/review.js';
	import { monday, today, dayLabel, duration } from './period.js';
	import PeriodPicker from './PeriodPicker.svelte';
	import ReviewerCounts from './ReviewerCounts.svelte';
	import DailyChart from '../admin/DailyChart.svelte';

	let from = $state(monday(today()));
	let to = $state(today());
	let data = $state(null);
	let counts = $state(null);
	let loading = $state(false);
	let error = $state('');

	const query = $derived(`from=${from}&to=${to}`);

	async function load() {
		loading = true;
		error = '';
		try {
			[data, counts] = await Promise.all([get(`/review/insights?${query}`), get(`/review/reviewer-stats?${query}`)]);
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}
	onMount(load);

	const plural = (n, word) => `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`;
	const pct = (n, of) => (of ? `${Math.round((n / of) * 100)}%` : '–');
	const period = $derived(data ? (data.from === data.to ? dayLabel(data.from) : `${dayLabel(data.from)} to ${dayLabel(data.to)}`) : '');

	const reviews = $derived(data ? data.days.reduce((n, d) => n + d.reviews, 0) : 0);
	const first = $derived(data?.decisions[0]);
	const firstDecided = $derived(first ? first.passed + first.rejected : 0);

	// Every step a ship shipped in the period reached, as a share of them all.
	const STEPS = [
		['shipped', 'Shipped'],
		['first_reviewed', 'First reviewed'],
		['passed_first_review', 'Approved at the first review'],
		['reached_hq', 'Reached HQ'],
		['signed_off', 'Signed off']
	];
	const funnel = $derived(data?.funnel);

	const me = $derived(session.me?.id);
	const admin = $derived(isAdmin());
</script>

<div class="stack">
	<PeriodPicker bind:from bind:to onchange={load} />

	{#if error}<p class="callout warn">{error}</p>{/if}

	{#if data}
		<div class="stack" class:loading>
			<section class="frame now" aria-label="The queue and the period">
				<div class="part">
					<h2>Waiting now</h2>
					<ul class="queue">
						{#each data.waiting as w (w.stage)}
							<li>
								<a class="stage" href="/review?stage={w.stage}">{STAGES[w.stage].label}</a>
								<strong>{w.waiting}</strong>
								<span class="small muted">
									{#if w.waiting}
										oldest {duration(w.oldest_wait_seconds)}: <a href="/review/{w.oldest_id}">{w.oldest_project}</a>.
										Median wait {duration(w.median_wait_seconds)}.
									{:else}
										Nothing waiting.
									{/if}
								</span>
							</li>
						{/each}
					</ul>
				</div>

				<div class="part">
					<h2>In this period</h2>
					<ul class="numbers">
						<li title="Decisions on ships, by everyone"><strong>{reviews.toLocaleString()}</strong><span>Reviews by {data.reviewers.toLocaleString()} {data.reviewers === 1 ? 'person' : 'people'}</span></li>
						<li title="Rejections out of every first-review decision"><strong>{pct(first.rejected, firstDecided)}</strong><span>Rejected at the first review</span></li>
						<li title="Hours HQ signed off"><strong>{hours(data.seconds_signed_off)}</strong><span>Signed off</span></li>
						<li title="Median time from shipping to the first decision, for ships first reviewed in the period"><strong>{duration(data.median_first_review_seconds)}</strong><span>Shipped to first review</span></li>
						<li title="Median time from shipping to HQ's sign-off, for ships signed off in the period"><strong>{duration(data.median_sign_off_seconds)}</strong><span>Shipped to signed off</span></li>
					</ul>
				</div>

				<div class="part">
					<div class="head"><h2>Reviews each day</h2><span class="small muted">{plural(reviews, 'review')}</span></div>
					<DailyChart days={data.days} value={(d) => d.reviews} label="reviews" />
				</div>
			</section>

			<section class="frame two" aria-label="Where ships went">
				<div class="part">
					<div class="head">
						<h2>Where ships went</h2>
						<span class="small muted">{plural(funnel.shipped, 'ship')} shipped {period}</span>
					</div>
					{#if funnel.shipped}
						<ol class="funnel">
							{#each STEPS as [key, label] (key)}
								<li>
									<span class="label">{label}</span>
									<span class="track"><span class="bar" style:width="{(funnel[key] / funnel.shipped) * 100}%"></span></span>
									<span class="value">{funnel[key].toLocaleString()} <span class="muted">{pct(funnel[key], funnel.shipped)}</span></span>
								</li>
							{/each}
						</ol>
						<p class="small muted">
							Of them, {funnel.rejected.toLocaleString()} rejected, {funnel.in_review.toLocaleString()} still in review, and
							{funnel.fraud_checked.toLocaleString()} went through the fraud check. A ship counts at every step it reached, even if it was
							sent back later.
						</p>
					{:else}
						<p class="small muted">Nothing was shipped in this period.</p>
					{/if}
				</div>

				<div class="part">
					<h2>Rejections by stage</h2>
					<table class="rates">
						<thead><tr><th>Stage</th><th class="num">Decided</th><th class="num">Rejected</th><th class="num">Rate</th><th class="num">Sent back</th></tr></thead>
						<tbody>
							{#each data.decisions as d (d.stage)}
								{@const decided = d.passed + d.rejected + d.sent_back}
								<tr>
									<td>{STAGES[d.stage].label}</td>
									<td class="num">{decided}</td>
									<td class="num">{d.rejected}</td>
									<td class="num rate">{pct(d.rejected, decided)}</td>
									<td class="num">{d.stage === 'review' ? '' : d.sent_back}</td>
								</tr>
							{/each}
						</tbody>
					</table>
					<p class="small muted">Decisions made {period}. Sending a ship back from the fraud check or HQ returns it to the review queue.</p>
				</div>
			</section>

			{#if counts}
				<section class="card table-wrap" aria-label="Everyone's reviews">
					<div class="summary">
						<h2>Everyone's reviews</h2>
						<span class="small muted">{admin ? 'Open anyone to see their reviews.' : 'Open your own row to see your reviews.'}</span>
					</div>
					<ReviewerCounts reviewers={counts.reviewers} {query} base="/review" canOpen={(r) => admin || r.id === me} />
				</section>
			{/if}
		</div>
	{/if}
</div>

<style>
	.loading { opacity: 0.55; transition: opacity 0.2s; }
	h2 { font-size: 24px; line-height: 1.1; margin: 0; }
	p { margin: 0; }

	/* One surface per group, its parts divided by rules, like the dashboard. */
	.frame {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		background: var(--row-bg);
		border-radius: var(--radius);
		overflow: hidden;
	}
	.part { display: flex; flex-direction: column; gap: 10px; padding: 14px 18px 16px; min-width: 0; }
	.part + .part { border-top: 1px solid var(--deep); }
	@media (min-width: 900px) {
		.two { grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); }
		.two .part + .part { border-top: 0; border-left: 1px solid var(--deep); }
	}
	@media (min-width: 1100px) {
		.now { grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr) minmax(0, 1fr); }
		.now .part + .part { border-top: 0; border-left: 1px solid var(--deep); }
	}
	.head { display: flex; align-items: baseline; justify-content: space-between; gap: 4px 10px; flex-wrap: wrap; }

	/* Waiting now: a row per stage, the count beside its name. */
	.queue { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 10px; }
	.queue li { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 0 12px; align-items: baseline; }
	.queue .stage { font-size: 20px; }
	.queue strong { font-size: 28px; font-weight: 400; line-height: 1; font-variant-numeric: tabular-nums; }
	.queue .small { grid-column: 1 / -1; overflow-wrap: anywhere; }

	/* In this period: numbers over their labels, as many across as fit. */
	.numbers { list-style: none; margin: 0; padding: 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 120px), 1fr)); gap: 6px; }
	.numbers li { display: flex; flex-direction: column; justify-content: center; gap: 2px; padding: 8px 10px; border-radius: 10px; background: var(--deep); min-width: 0; }
	.numbers strong { font-size: 28px; font-weight: 400; line-height: 1; font-variant-numeric: tabular-nums; }
	.numbers span { font-size: 16px; line-height: 1.15; color: var(--muted); text-wrap: balance; }

	/* Where ships went: one bar per step, all out of the ships shipped. */
	.funnel { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
	.funnel li { display: grid; grid-template-columns: minmax(8em, 11em) minmax(0, 1fr) 6.5em; align-items: center; gap: 12px; }
	.track { height: 14px; background: var(--deep); border-radius: 4px; overflow: hidden; }
	.bar { display: block; height: 100%; min-width: 2px; background: var(--info); border-radius: 0 4px 4px 0; }
	.value { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
	@media (max-width: 560px) {
		.funnel li { grid-template-columns: minmax(0, 1fr) auto; }
		.track { grid-column: 1 / -1; grid-row: 2; }
	}

	.rates { width: 100%; font-size: 20px; }
	.rates th { background: none; color: var(--muted); font-size: 17px; font-weight: 400; border-bottom: 1px solid var(--deep); }
	.rates th, .rates td { padding: 4px 8px; }
	.rates th:first-child, .rates td:first-child { padding-left: 0; }
	.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
	.rate { font-size: 22px; }

	.summary { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 4px 10px; padding: 14px 16px; }
</style>
