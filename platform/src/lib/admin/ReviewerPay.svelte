<script>
	// Paying reviewers per review (GET and POST /superadmin/reviewer-pay,
	// backend/api/reviewpay.go). Pick days of unpaid reviews, set a rate, and
	// pay: in tickets, which go to the reviewer, or (superadmins only) in US
	// dollars, which only records it. Either way the reviews are marked paid
	// and can't be paid again. A review is a decision on a ship, as in Admin
	// review stats; days run midnight to midnight Eastern.
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';
	import { session } from '../session.svelte.js';
	import { dayLabel, today, shift } from '../review/period.js';
	import { date } from '../format.js';

	let data = $state(null);
	let error = $state('');
	let message = $state('');
	let busy = $state(false);

	let currency = $state('tickets'); // or usd, for superadmins
	// The rate per review: whole tickets, or dollars (cents to the backend).
	let rates = $state({ tickets: 5, usd: 2.5 });
	// The days picked for each reviewer: { [id]: ['2026-11-17', …] }.
	let picked = $state({});
	let open = $state({}); // reviewers whose days are shown
	let from = $state(shift(today(), -6));
	let to = $state(today());

	onMount(() => {
		try {
			const saved = JSON.parse(localStorage.getItem('crossroads.reviewerPayRates') ?? 'null');
			if (saved) rates = { ...rates, ...saved };
		} catch {}
		load();
	});
	$effect(() => {
		const r = { ...rates };
		try {
			localStorage.setItem('crossroads.reviewerPayRates', JSON.stringify(r));
		} catch {}
	});

	async function load() {
		error = '';
		try {
			data = await get('/superadmin/reviewer-pay');
			if (!data.can_pay_in_usd) currency = 'tickets';
		} catch (e) {
			error = e.message;
		}
	}

	const me = $derived(session.me?.id);
	const payable = (r) => r.id !== me;
	const unpaidOf = (r, days) => r.days.filter((d) => days.includes(d.day)).reduce((n, d) => n + d.unpaid, 0);

	// Picking: a day, a reviewer's every unpaid day, everyone's, or everyone's
	// between two days.
	function pickWhere(test) {
		const next = {};
		for (const r of data.reviewers) {
			if (!payable(r)) continue;
			const days = r.days.filter((d) => d.unpaid > 0 && test(d.day)).map((d) => d.day);
			if (days.length) next[r.id] = days;
		}
		picked = next;
		message = '';
	}
	const pickAllUnpaid = () => pickWhere(() => true);
	const pickBetween = () => pickWhere((day) => (!from || day >= from) && (!to || day <= to));
	function toggleDay(r, day) {
		const days = picked[r.id] ?? [];
		picked[r.id] = days.includes(day) ? days.filter((d) => d !== day) : [...days, day];
		message = '';
	}
	function toggleReviewer(r) {
		const unpaid = r.days.filter((d) => d.unpaid > 0).map((d) => d.day);
		picked[r.id] = (picked[r.id]?.length ?? 0) === unpaid.length ? [] : unpaid;
		message = '';
	}

	// What's picked, per reviewer, and what it comes to.
	const chosen = $derived(
		(data?.reviewers ?? [])
			.map((r) => ({ r, days: picked[r.id] ?? [], reviews: unpaidOf(r, picked[r.id] ?? []) }))
			.filter((c) => c.reviews > 0)
	);
	const reviews = $derived(chosen.reduce((n, c) => n + c.reviews, 0));
	// The rate as the backend takes it: tickets, or cents.
	const rate = $derived(currency === 'usd' ? Math.round((Number(rates.usd) || 0) * 100) : Math.floor(Number(rates.tickets) || 0));
	const rateOk = $derived(rate >= 1 && rate <= (currency === 'usd' ? 100_000 : 1000));
	const dollars = (cents) => `$${(cents / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
	const amount = (n, cur = currency) => (cur === 'usd' ? dollars(n) : `${n.toLocaleString()} ticket${n === 1 ? '' : 's'}`);
	const plural = (n, word) => `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`;

	async function pay() {
		if (!reviews || !rateOk) return;
		const total = amount(reviews * rate);
		const who = chosen.length === 1 ? chosen[0].r.name : plural(chosen.length, 'reviewer');
		const ask =
			currency === 'usd'
				? `Record ${total} paid to ${who} for ${plural(reviews, 'review')}? Nothing is sent: pay them yourself. The reviews are marked paid.`
				: `Give ${who} ${total} for ${plural(reviews, 'review')}? The reviews are marked paid.`;
		if (!confirm(ask)) return;
		busy = true;
		error = message = '';
		try {
			await api('POST', '/superadmin/reviewer-pay', {
				currency,
				rate,
				payments: chosen.map((c) => ({ reviewer_id: c.r.id, days: c.days, reviews: c.reviews }))
			});
			message = currency === 'usd' ? `Recorded ${total} paid to ${who}.` : `Gave ${who} ${total}.`;
			picked = {};
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
			await load();
		}
	}
</script>

<div class="stack">
	<p class="small muted">
		Pay reviewers per review. A review is one decision on a ship, as in Admin review stats, and days run midnight to midnight
		Eastern. Pick the days to pay for, set a rate, and pay: the reviews are marked paid and can't be paid again.
		{#if data?.can_pay_in_usd}Tickets go to the reviewer; dollars are only recorded, and only superadmins see them.{/if}
	</p>

	{#if error}<p class="callout warn" role="alert">{error}</p>{/if}
	{#if message}<p class="callout good" role="status">{message}</p>{/if}

	{#if data}
		<section class="card controls" aria-label="How to pay">
			<div class="row group">
				{#if data.can_pay_in_usd}
					<div class="toggle" role="group" aria-label="Pay in">
						<button aria-pressed={currency === 'tickets'} onclick={() => (currency = 'tickets')}>Tickets</button>
						<button aria-pressed={currency === 'usd'} onclick={() => (currency = 'usd')}>US dollars</button>
					</div>
				{/if}
				<label class="rate">
					{#if currency === 'usd'}
						<span>$</span><input type="number" min="0.01" max="1000" step="0.01" bind:value={rates.usd} aria-label="Dollars per review" />
					{:else}
						<input type="number" min="1" max="1000" step="1" bind:value={rates.tickets} aria-label="Tickets per review" /><span>tickets</span>
					{/if}
					<span class="muted">per review</span>
				</label>
			</div>
			<div class="row group">
				<button class="btn small" onclick={pickAllUnpaid}>Select all unpaid</button>
				<span class="between">
					<label>From <input type="date" bind:value={from} /></label>
					<label>to <input type="date" bind:value={to} /></label>
					<button class="btn small ghost" onclick={pickBetween}>Select unpaid in these days</button>
				</span>
				{#if reviews}<button class="btn small ghost" onclick={() => (picked = {})}>Clear</button>{/if}
			</div>
		</section>

		{#if data.reviewers.length}
			<div class="card table-wrap">
				<table>
					<thead>
						<tr>
							<th class="pick"><span class="sr-only">Select</span></th>
							<th>Reviewer</th>
							<th class="num">Reviews</th>
							<th class="num">Unpaid</th>
							<th class="num">Paid in tickets</th>
							{#if data.can_pay_in_usd}<th class="num">Paid in dollars</th>{/if}
							<th class="num">Selected</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each data.reviewers as r (r.id)}
							{@const unpaidDays = r.days.filter((d) => d.unpaid > 0).length}
							{@const mine = picked[r.id] ?? []}
							{@const selected = unpaidOf(r, mine)}
							<tr class:chosen={selected > 0}>
								<td class="pick">
									{#if payable(r) && unpaidDays}
										<input
											type="checkbox"
											checked={mine.length > 0 && mine.length === unpaidDays}
											indeterminate={mine.length > 0 && mine.length < unpaidDays}
											onchange={() => toggleReviewer(r)}
											aria-label="Select all of {r.name}'s unpaid reviews"
										/>
									{/if}
								</td>
								<td>{r.name}{#if !payable(r)} <span class="badge" title="You can't pay yourself">You</span>{/if}</td>
								<td class="num">{r.reviews.toLocaleString()}</td>
								<td class="num" class:muted={!r.unpaid}>{r.unpaid.toLocaleString()}</td>
								<td class="num">{r.paid_tickets ? r.paid_tickets.toLocaleString() : '–'}</td>
								{#if data.can_pay_in_usd}<td class="num">{r.paid_cents ? dollars(r.paid_cents) : '–'}</td>{/if}
								<td class="num">{selected ? selected.toLocaleString() : ''}</td>
								<td>
									<button class="btn small ghost" aria-expanded={!!open[r.id]} onclick={() => (open[r.id] = !open[r.id])}>{open[r.id] ? 'Hide days' : 'Days'}</button>
								</td>
							</tr>
							{#if open[r.id]}
								<tr class="days-row">
									<td colspan={data.can_pay_in_usd ? 8 : 7}>
										<ul class="days">
											{#each r.days as d (d.day)}
												<li>
													<label class:paid={!d.unpaid}>
														<input type="checkbox" disabled={!d.unpaid || !payable(r)} checked={mine.includes(d.day)} onchange={() => toggleDay(r, d.day)} />
														<span class="day">{dayLabel(d.day)}</span>
														<span class="small muted">{d.unpaid ? `${d.unpaid} of ${d.reviews} unpaid` : `${plural(d.reviews, 'review')}, paid`}</span>
													</label>
												</li>
											{/each}
										</ul>
									</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			</div>
		{:else}
			<div class="card dashed"><p class="muted">Nobody has reviewed anything yet.</p></div>
		{/if}

		<div class="paybar card" aria-live="polite">
			<span>
				{#if reviews}
					<strong>{plural(reviews, 'review')}</strong> from {plural(chosen.length, 'reviewer')}, at
					{currency === 'usd' ? dollars(rate) : amount(rate)} each:
					<strong>{amount(reviews * rate)}</strong>
				{:else}
					<span class="muted">Select some unpaid reviews to pay for.</span>
				{/if}
			</span>
			<button class="btn primary" disabled={busy || !reviews || !rateOk} onclick={pay}>
				{busy ? 'Paying…' : !reviews ? 'Pay' : currency === 'usd' ? `Record ${amount(reviews * rate)} paid` : `Pay ${amount(reviews * rate)}`}
			</button>
		</div>
		{#if reviews && !rateOk}<p class="small warn-text">Set a rate between {currency === 'usd' ? '$0.01 and $1,000' : '1 and 1,000 tickets'} per review.</p>{/if}

		<section class="card table-wrap" aria-labelledby="payments-head">
			<div class="summary"><h2 id="payments-head">Payments</h2><span class="small muted">The latest 100.</span></div>
			{#if data.payments.length}
				<table>
					<thead><tr><th>When</th><th>Reviewer</th><th class="num">Reviews</th><th class="num">Rate</th><th class="num">Paid</th><th>By</th><th>For</th></tr></thead>
					<tbody>
						{#each data.payments as p (p.id)}
							<tr>
								<td class="small">{date(p.created_at)}</td>
								<td>{p.reviewer_name}</td>
								<td class="num">{p.reviews.toLocaleString()}</td>
								<td class="num">{p.currency === 'usd' ? dollars(p.rate) : p.rate.toLocaleString()}</td>
								<td class="num">{amount(p.amount, p.currency)}{#if p.currency === 'usd'} <span class="badge tiny" title="Recorded, not sent">Recorded</span>{/if}</td>
								<td class="small">{p.paid_by_name}</td>
								<td class="small muted">{p.note}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{:else}
				<p class="muted empty">Nobody's been paid yet.</p>
			{/if}
		</section>
	{/if}
</div>

<style>
	h2 { font-size: 24px; margin: 0; }
	.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
	.pick { width: 40px; text-align: center; }
	.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
	tr.chosen > td { background: var(--deep); }

	.controls { display: flex; flex-direction: column; gap: 10px; padding: 12px 16px; }
	.group { flex-wrap: wrap; align-items: center; gap: 8px 14px; }
	.rate { display: inline-flex; align-items: center; gap: 8px; }
	.rate input { width: 6em; padding: 4px 10px; }
	.between { display: inline-flex; flex-wrap: wrap; align-items: center; gap: 8px; }
	.between label { display: inline-flex; align-items: center; gap: 6px; }
	.between input { width: auto; padding: 4px 10px; font-size: 20px; }

	.toggle { display: inline-flex; padding: 2px; border-radius: 999px; background: var(--deep); }
	.toggle button { padding: 2px 14px; border: 0; border-radius: 999px; background: transparent; color: var(--muted); font: inherit; font-size: 18px; cursor: pointer; }
	.toggle button[aria-pressed='true'] { background: var(--body); color: var(--text); }
	.toggle button:focus-visible { outline: 2px solid var(--muted); outline-offset: 2px; }

	.days-row > td { background: var(--deep); padding: 8px 16px 14px; }
	.days { list-style: none; margin: 0; padding: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 230px), 1fr)); gap: 6px; }
	.days label { display: grid; grid-template-columns: auto 1fr; align-items: center; gap: 0 10px; padding: 6px 10px; border-radius: 10px; background: var(--row-bg); cursor: pointer; }
	.days label.paid { opacity: 0.6; cursor: default; }
	.days .small { grid-column: 2; }

	/* What's selected and the button, kept in view while picking. */
	.paybar { position: sticky; bottom: 12px; z-index: 3; display: flex; flex-direction: row; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 16px; padding: 12px 16px; box-shadow: 0 4px 18px rgb(0 0 0 / 0.45); }
	.warn-text { color: var(--warn); margin: 0; }

	.summary { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 4px 10px; padding: 14px 16px; }
	.empty { padding: 0 16px 16px; }
	.tiny { font-size: 15px; padding: 0 8px; }
</style>
