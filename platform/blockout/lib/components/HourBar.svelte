<script>
	// The one progress bar everything hangs off: approved hours toward the goal,
	// stacked with hours in review and hours not shipped yet.
	import { PROGRAM } from '#lib/data.js';
	let { hours, goal = PROGRAM.goalHours, compact = false } = $props();

	const pct = (h) => Math.min((h / goal) * 100, 100);
	const a = $derived(pct(hours.approved));
	const r = $derived(Math.min(pct(hours.review), 100 - a));
	const u = $derived(Math.min(pct(hours.unshipped), 100 - a - r));
	const left = $derived(Math.max(goal - hours.approved, 0));
</script>

<div class="hourbar">
	{#if !compact}
		<div class="row between">
			<strong>{hours.approved} / {goal} approved hours</strong>
			<span class="muted">{left > 0 ? `${left}h to qualify` : 'Qualified!'}</span>
		</div>
	{/if}
	<div class="bar" role="img" aria-label="{hours.approved} of {goal} hours approved">
		<span class="seg approved" style="width: {a}%"></span>
		<span class="seg review" style="width: {r}%"></span>
		<span class="seg unshipped" style="width: {u}%"></span>
	</div>
	{#if !compact}
		<div class="ticks"><span>0</span><span>15</span><span>30</span><span>45</span><span>60</span></div>
		<div class="legend">
			<span><i class="approved"></i>Approved {hours.approved}h</span>
			<span><i class="review"></i>In review {hours.review}h</span>
			<span><i class="unshipped"></i>Not shipped {hours.unshipped}h</span>
		</div>
	{/if}
</div>
