<script>
	// How many reviews each person made in a period, for paying reviewers
	// (GET /admin/reviewer-stats and /admin/reviews). A review is one decision
	// on a ship; see backend/db/queries/reviewstats.sql. Days run midnight to
	// midnight Eastern, and a finished period's counts never change.
	import { onMount } from 'svelte';
	import { get } from '../api.js';
	import { monday, today, dayLabel } from '../review/period.js';
	import PeriodPicker from '../review/PeriodPicker.svelte';
	import ReviewerCounts from '../review/ReviewerCounts.svelte';

	let from = $state(monday(today()));
	let to = $state(today());
	let data = $state(null);
	let loading = $state(false);
	let error = $state('');

	const query = $derived(`from=${from}&to=${to}`);

	async function load() {
		loading = true;
		error = '';
		try {
			data = await get(`/admin/reviewer-stats?${query}`);
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}
	onMount(load);
</script>

<div class="stack">
	<PeriodPicker bind:from bind:to onchange={load} />

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
			<ReviewerCounts reviewers={data.reviewers} {query} base="/admin" />
		</div>
	{/if}
</div>

<style>
	.loading { opacity: 0.55; transition: opacity 0.2s; }

	.summary { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; padding: 14px 16px; }
	.downloads { display: flex; flex-wrap: wrap; gap: 8px; }
</style>
