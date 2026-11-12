<script>
	// The top of the review queue: your reviews, how the queue is doing this
	// week, and the leaderboard. Counts come from GET /review/stats, which
	// counts a review the same way the admin's pay view does: one decision
	// on a ship (see backend/db/queries/reviewstats.sql).
	import { onMount } from 'svelte';
	import { get } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import { hours } from '#lib/format.js';

	let data = $state(null);
	let error = $state('');
	let board = $state('week'); // or 'all_time'
	const SHOWN = 8;

	onMount(async () => {
		try {
			data = await get('/review/stats');
		} catch (e) {
			error = e.message;
		}
	});

	const me = $derived(session.me?.id);
	const rankIn = (list) => {
		const i = list.findIndex((r) => r.id === me);
		return i < 0 ? null : { place: i + 1, row: list[i] };
	};
	const week = $derived(data ? rankIn(data.week) : null);
	const allTime = $derived(data ? rankIn(data.all_time) : null);
	const rows = $derived(data ? data[board] : []);
	const shown = $derived(rows.slice(0, SHOWN));
	const mine = $derived(rows.findIndex((r) => r.id === me));

	// "Nov 9 – 15", Monday to Sunday in Eastern time.
	const weekLabel = $derived.by(() => {
		if (!data) return '';
		const opts = { month: 'short', day: 'numeric', timeZone: 'America/New_York' };
		const start = new Date(data.week_start).toLocaleDateString(undefined, opts);
		const end = new Date(new Date(data.week_end) - 1).toLocaleDateString(undefined, opts);
		return `${start} – ${end.replace(start.split(' ')[0] + ' ', '')}`;
	});

	// A wait as "45m", "6h" or "2d 4h".
	function wait(seconds) {
		if (seconds == null) return '–';
		const m = Math.round(seconds / 60);
		if (m < 60) return `${m}m`;
		const h = Math.round(m / 60);
		if (h < 48) return `${h}h`;
		return `${Math.floor(h / 24)}d ${h % 24}h`;
	}
	// 1st, 2nd, 3rd, 4th … 11th, 12th, 13th … 21st.
	function ordinal(n) {
		const suffix = ['th', 'st', 'nd', 'rd'];
		const v = n % 100;
		return n + (suffix[(v - 20) % 10] || suffix[v] || suffix[0]);
	}
</script>

{#if error}
	<p class="callout warn small">Couldn't load the review stats: {error}</p>
{:else}
	<section class="dash" class:loading={!data} aria-label="Review stats">
		<div class="part you">
			<h2>You</h2>
			<div class="big">
				<strong>{week?.row.reviews ?? 0}</strong>
				<span>review{week?.row.reviews === 1 ? '' : 's'} this week</span>
			</div>
			<p class="small muted">
				{#if week}{ordinal(week.place)} of {data.week.length} this week{:else if data}None yet this week{:else}&nbsp;{/if}
			</p>
			<dl class="facts">
				<div><dt>All time</dt><dd>{allTime?.row.reviews ?? 0}</dd></div>
				<div><dt>Hours first-reviewed</dt><dd>{hours(allTime?.row.seconds_first_reviewed ?? 0)}</dd></div>
			</dl>
		</div>

		<div class="part">
			<h2>This week <span class="muted when">{weekLabel}</span></h2>
			<dl class="facts grid">
				<div><dt>Reviews, everyone</dt><dd>{data?.pace.reviewed ?? '–'}</dd></div>
				<div><dt>Ships in</dt><dd>{data?.pace.shipped ?? '–'}</dd></div>
				<div><dt>Hours signed off</dt><dd>{data ? hours(data.pace.seconds_approved) : '–'}</dd></div>
				<div>
					<dt>Waiting now</dt>
					<dd>{data?.pace.waiting ?? '–'}{#if data?.pace.oldest_wait_seconds != null} <span class="muted sub">oldest {wait(data.pace.oldest_wait_seconds)}</span>{/if}</dd>
				</div>
				<div class="wide"><dt>Median wait for a first review, last 7 days</dt><dd>{wait(data?.pace.median_wait_seconds)}</dd></div>
			</dl>
		</div>

		<div class="part board">
			<div class="board-head">
				<h2>Leaderboard</h2>
				<div class="toggle" role="group" aria-label="Leaderboard period">
					<button aria-pressed={board === 'week'} onclick={() => (board = 'week')}>This week</button>
					<button aria-pressed={board === 'all_time'} onclick={() => (board = 'all_time')}>All time</button>
				</div>
			</div>
			{#if rows.length}
				<ol class="leaders">
					{#each shown as r, i (r.id)}
						<li class:you={r.id === me}>
							<span class="place">{i + 1}</span>
							<span class="name">{r.name}{#if r.id === me} <span class="badge">You</span>{/if}</span>
							<span class="count">{r.reviews}</span>
						</li>
					{/each}
					{#if mine >= SHOWN}
						<li class="you gap">
							<span class="place">{mine + 1}</span>
							<span class="name">{rows[mine].name} <span class="badge">You</span></span>
							<span class="count">{rows[mine].reviews}</span>
						</li>
					{/if}
				</ol>
			{:else if data}
				<p class="small muted">No reviews {board === 'week' ? 'yet this week' : 'yet'}. The first one tops the board.</p>
			{/if}
		</div>
	</section>
	<p class="footnote muted">A review is one decision on a ship: approving, rejecting, passing the fraud check, signing off or sending back. Weeks run Monday to Sunday, Eastern time.</p>
{/if}

<style>
	.dash {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		background: var(--row-bg);
		border-radius: var(--radius);
		overflow: hidden;
		transition: opacity 0.2s;
	}
	.loading { opacity: 0.55; }
	@media (min-width: 900px) {
		.dash { grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr) minmax(0, 1.1fr); }
		.part + .part { border-left: 1px solid var(--deep); }
	}
	@media (max-width: 899px) {
		.part + .part { border-top: 1px solid var(--deep); }
	}
	.part { display: flex; flex-direction: column; gap: 10px; padding: 18px 22px; min-width: 0; }
	h2 { font-size: 30px; line-height: 1.1; }
	.when { font-size: 20px; margin-left: 6px; }

	.big { display: flex; align-items: baseline; gap: 10px; flex-wrap: wrap; }
	.big strong { font-size: 64px; font-weight: 400; line-height: 0.9; font-variant-numeric: tabular-nums; }
	.big span { font-size: 22px; color: var(--muted); }
	.you > p { margin: 0; }

	.facts { margin: 0; display: flex; flex-direction: column; gap: 6px; }
	.facts.grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 18px; }
	.facts .wide { grid-column: 1 / -1; }
	.facts dt { font-size: 18px; color: var(--muted); }
	.facts dd { margin: 0; font-size: 30px; line-height: 1.1; font-variant-numeric: tabular-nums; }
	.facts .sub { font-size: 18px; margin-left: 8px; }
	.you .facts { flex-direction: row; gap: 24px; margin-top: auto; }

	.board-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
	.toggle { display: inline-flex; padding: 2px; border-radius: 999px; background: var(--deep); }
	.toggle button { padding: 2px 14px; border: 0; border-radius: 999px; background: transparent; color: var(--muted); font: inherit; font-size: 18px; cursor: pointer; }
	.toggle button[aria-pressed='true'] { background: var(--body); color: var(--text); }

	.leaders { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
	.leaders li {
		display: grid;
		grid-template-columns: 1.8em minmax(0, 1fr) auto;
		align-items: baseline;
		gap: 10px;
		padding: 4px 8px;
		border-radius: 8px;
		font-size: 22px;
	}
	.leaders li.you { background: var(--deep); }
	.leaders li.gap { margin-top: 6px; position: relative; }
	.leaders li.gap::before { content: '⋯'; position: absolute; top: -18px; left: 12px; color: var(--muted); font-size: 16px; }
	.place { color: var(--muted); font-variant-numeric: tabular-nums; text-align: right; }
	.name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.count { font-variant-numeric: tabular-nums; }
	.leaders li:nth-child(1) .place { color: var(--head); }

	.footnote { margin: 6px 4px 20px; font-size: 17px; }
</style>
