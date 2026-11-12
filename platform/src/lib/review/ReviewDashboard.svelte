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
	const SHOWN = 3;

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

	// The week as a bar per day, Monday first. Days still to come stay empty.
	const days = $derived(data?.days ?? []);
	const most = $derived(Math.max(1, ...days.map((d) => d.reviews)));
	const weekTotal = $derived(days.reduce((n, d) => n + d.reviews, 0));
	const dayName = (day, style) => new Date(day + 'T12:00:00Z').toLocaleDateString(undefined, { weekday: style, timeZone: 'UTC' });
	const dayLabel = (day) => new Date(day + 'T12:00:00Z').toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric', timeZone: 'UTC' });
	const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;
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
</script>

{#if error}
	<p class="callout warn small">Couldn't load the review stats: {error}</p>
{:else}
	<section class="dash" class:loading={!data} aria-label="Review stats">
		<div class="part">
			<h2>You</h2>
			<ul class="mine">
				<li title="Decisions on ships: approving, rejecting, passing, signing off or sending back"><strong>{week?.row.reviews ?? 0}</strong><span>Reviews this week</span></li>
				<li><strong>{allTime?.row.reviews ?? 0}</strong><span>Reviews all time</span></li>
				<li title="Hours claimed by the ships you first-reviewed"><strong>{hours(allTime?.row.seconds_first_reviewed ?? 0)}</strong><span>Hours all time</span></li>
			</ul>
		</div>

		<div class="part">
			<div class="week-head">
				<h2>This week</h2>
				{#if data}
					<span class="muted small" title={weekLabel}>{plural(weekTotal, 'review')} · {data.pace.waiting ? `${data.pace.waiting} waiting, oldest ${wait(data.pace.oldest_wait_seconds)}` : 'none waiting'}</span>
				{/if}
			</div>
			<!-- One series: reviews by everyone each day. Hover a day for its count. -->
			<div class="chart" aria-hidden="true">
				{#each days as d (d.day)}
					{@const future = d.day > data.today}
					<div class="col" class:future class:today={d.day === data.today} title={future ? dayLabel(d.day) : `${dayLabel(d.day)}: ${plural(d.reviews, 'review')}`}>
						<div class="plot">
							{#if d.reviews}
								<div class="bar" style:height="{(d.reviews / most) * 100}%">
									{#if d.day === data.today}<span class="value">{d.reviews}</span>{/if}
								</div>
							{/if}
						</div>
						<span class="day">{dayName(d.day, 'narrow')}</span>
					</div>
				{/each}
			</div>
			<!-- Hidden in a wrapper, not on the table: Firefox draws a table's
			     caption outside the table's own box, where it would show. -->
			<div class="visually-hidden">
				<table>
					<caption>Reviews by everyone each day this week</caption>
					<tbody>{#each days as d (d.day)}<tr><th scope="row">{dayLabel(d.day)}</th><td>{d.reviews}</td></tr>{/each}</tbody>
				</table>
			</div>
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
							<span class="name" title={r.name}>{r.name}</span>
							<span class="tag">{#if r.id === me}<span class="badge">You</span>{/if}</span>
							<span class="count">{r.reviews}</span>
						</li>
					{/each}
					{#if mine >= SHOWN}
						<li class="you gap">
							<span class="place">{mine + 1}</span>
							<span class="name" title={rows[mine].name}>{rows[mine].name}</span>
							<span class="tag"><span class="badge">You</span></span>
							<span class="count">{rows[mine].reviews}</span>
						</li>
					{/if}
				</ol>
			{:else if data}
				<p class="small muted">No reviews {board === 'week' ? 'yet this week' : 'yet'}.</p>
			{/if}
		</div>
	</section>
{/if}

<style>
	.dash {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		margin-bottom: 16px;
		background: var(--row-bg);
		border-radius: var(--radius);
		overflow: hidden;
		transition: opacity 0.2s;
	}
	.loading { opacity: 0.55; }
	.part + .part { border-top: 1px solid var(--deep); }
	/* Two columns with the leaderboard under them, then three when there's room. */
	@media (min-width: 700px) {
		.dash { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
		.part:nth-child(2) { border-top: 0; border-left: 1px solid var(--deep); }
		.board { grid-column: 1 / -1; }
	}
	@media (min-width: 1100px) {
		.dash { grid-template-columns: repeat(3, minmax(0, 1fr)); }
		.part + .part { border-top: 0; border-left: 1px solid var(--deep); }
		.board { grid-column: auto; }
	}
	.part { display: flex; flex-direction: column; gap: 8px; padding: 12px 18px 14px; min-width: 0; }
	h2 { font-size: 24px; line-height: 1.1; }

	/* You: three numbers side by side, each over its label. */
	/* Three across when they fit; fewer per row rather than squeezed. */
	.mine { list-style: none; margin: 0; padding: 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 104px), 1fr)); gap: 6px; flex: 1; }
	.mine li {
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: 2px;
		padding: 8px 10px;
		border-radius: 10px;
		background: var(--deep);
		min-width: 0;
	}
	.mine strong { font-size: 28px; font-weight: 400; line-height: 1; font-variant-numeric: tabular-nums; }
	.mine span { font-size: 16px; line-height: 1.15; color: var(--muted); text-wrap: balance; }

	/* This week: a bar per day, anchored to the baseline, rounded at the top. */
	.week-head { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
	.chart { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 2px; flex: 1; min-height: 76px; }
	.col { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
	/* Room above the tallest bar for today's number. */
	.plot {
		position: relative;
		flex: 1;
		display: flex;
		flex-direction: column;
		justify-content: flex-end;
		align-items: center;
		margin-top: 18px;
		border-bottom: 1px solid var(--muted);
	}
	.bar { position: relative; width: min(70%, 34px); min-height: 3px; background: var(--info); border-radius: 4px 4px 0 0; }
	.value { position: absolute; bottom: 100%; left: 50%; translate: -50% -2px; white-space: nowrap; }
	.col:hover .bar { filter: brightness(1.15); }
	.value { font-size: 18px; line-height: 1.2; font-variant-numeric: tabular-nums; }
	.day { text-align: center; font-size: 17px; color: var(--muted); }
	.today .day { color: var(--text); font-weight: 700; }
	.future .day { opacity: 0.45; }
	.future .plot { border-bottom-style: dashed; border-bottom-color: var(--deep); }
	.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }

	.board-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
	.toggle { display: inline-flex; padding: 2px; border-radius: 999px; background: var(--deep); }
	.toggle button { padding: 2px 14px; border: 0; border-radius: 999px; background: transparent; color: var(--muted); font: inherit; font-size: 18px; cursor: pointer; }
	.toggle button[aria-pressed='true'] { background: var(--body); color: var(--text); }

	.leaders { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
	.leaders li {
		display: grid;
		grid-template-columns: 1.8em minmax(0, 1fr) auto auto;
		align-items: baseline;
		gap: 10px;
		padding: 2px 8px;
		border-radius: 8px;
		font-size: 20px;
	}
	.leaders li.you { background: var(--deep); }
	.leaders li.gap { margin-top: 6px; position: relative; }
	.leaders li.gap::before { content: '⋯'; position: absolute; top: -18px; left: 12px; color: var(--muted); font-size: 16px; }
	.place { color: var(--muted); font-variant-numeric: tabular-nums; text-align: right; }
	.name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.tag .badge { font-size: 16px; padding: 0 8px; }
	.count { min-width: 2ch; text-align: right; }
	.count { font-variant-numeric: tabular-nums; }
	.leaders li:nth-child(1) .place { color: var(--head); }
</style>
