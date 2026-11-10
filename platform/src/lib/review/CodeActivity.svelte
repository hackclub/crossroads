<script>
	// The repo on GitHub and what happened in it during this ship, next to
	// Hackatime's time per day. Daily totals only: reviewers never see
	// heartbeats. The data comes from the review detail (backend/api/checks.go).
	import { hours, utcDay } from '#lib/format.js';

	let { github, githubError, days, windowStart, repoUrl, reship } = $props();

	const DAY = 86400000;
	const MAX_DAYS = 45;

	// Every UTC day from the window's start to the last day with anything, so
	// quiet days show as gaps.
	const columns = $derived.by(() => {
		const byDay = new Map((days ?? []).map((d) => [d.day, d]));
		const start = new Date(windowStart);
		let from = Date.UTC(start.getUTCFullYear(), start.getUTCMonth(), start.getUTCDate());
		const last = (days ?? []).reduce((m, d) => Math.max(m, Date.parse(d.day + 'T00:00:00Z')), from);
		from = Math.max(from, last - (MAX_DAYS - 1) * DAY);
		const out = [];
		for (let t = from; t <= last; t += DAY) {
			const day = new Date(t).toISOString().slice(0, 10);
			out.push(byDay.get(day) ?? { day, seconds: 0, commits: 0 });
		}
		return out;
	});
	const maxSeconds = $derived(Math.max(3600, ...columns.map((c) => c.seconds)));
	const maxCommits = $derived(Math.max(1, ...columns.map((c) => c.commits)));
	const busiest = $derived(Math.max(0, ...columns.map((c) => c.seconds)));
	const totalSeconds = $derived(columns.reduce((s, c) => s + c.seconds, 0));
	const totalCommits = $derived(columns.reduce((s, c) => s + c.commits, 0));

	const since = $derived(reship ? 'since the last approved ship' : 'since Crossroads started');
	let showAll = $state(false);
	const commits = $derived(github ? (showAll ? github.commits : github.commits.slice(0, 8)) : []);

	function shortDate(iso) {
		return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
	}
	function columnLabel(c) {
		return `${utcDay(c.day)}: ${hours(c.seconds)} on Hackatime, ${c.commits} commit${c.commits === 1 ? '' : 's'}`;
	}
</script>

<section class="card stack">
	<div class="row between">
		<h2>Code</h2>
		{#if github}
			<a class="small" href={github.url} target="_blank" rel="noreferrer">{github.owner}/{github.repo} ↗</a>
		{/if}
	</div>

	{#if github}
		<div class="row small facts">
			<span class="chip">Public</span>
			{#if github.language}<span class="chip">{github.language}</span>{/if}
			{#if github.fork}<span class="badge warn">Fork</span>{/if}
			{#if github.archived}<span class="badge">Archived</span>{/if}
			<span class="muted">Created {shortDate(github.created_at)} · last push {shortDate(github.pushed_at)} · {github.total_commits} commit{github.total_commits === 1 ? '' : 's'} all time{github.stars ? ` · ${github.stars} ★` : ''}</span>
		</div>

		<div class="tiles">
			<div class="tile">
				<span class="label">Commits</span>
				<strong>{github.commits.length}{github.truncated ? '+' : ''}</strong>
			</div>
			<div class="tile">
				<span class="label">Lines changed</span>
				{#if github.lines_counted}
					<strong>+{github.additions.toLocaleString()} <span class="muted">/ −{github.deletions.toLocaleString()}</span></strong>
				{:else}
					<strong class="muted">–</strong>
				{/if}
			</div>
			<div class="tile">
				<span class="label">By</span>
				<strong class="authors">
					{#each github.authors as a, i}{i ? ', ' : ''}{a.author} <span class="muted">({a.commits})</span>{:else}<span class="muted">–</span>{/each}
				</strong>
			</div>
		</div>
		<p class="small muted">On the default branch, {since} ({shortDate(github.since)}) until it shipped.{github.lines_note ? ' ' + github.lines_note : ''}</p>
	{:else if githubError}
		<p class="callout warn small">Couldn't ask GitHub: {githubError}</p>
	{:else if repoUrl}
		<p class="small muted">The repo isn't on GitHub, so there are no commit stats. Check it by hand.</p>
	{/if}

	{#if columns.length}
		<div class="stack-sm">
			<div class="strip-head small"><span>Hackatime hours per day</span><span class="muted">{hours(totalSeconds)} in all · busiest {hours(busiest)}</span></div>
			<div class="strip" role="img" aria-label="Hackatime hours per day, {since}">
				{#each columns as c (c.day)}
					<div class="col" title={columnLabel(c)}>
						<div class="bar" style="height: {(c.seconds / maxSeconds) * 100}%"></div>
					</div>
				{/each}
			</div>
			{#if github}
				<div class="strip-head small"><span>Commits per day</span><span class="muted">{totalCommits} in all · most {Math.max(0, ...columns.map((c) => c.commits))}</span></div>
				<div class="strip short" role="img" aria-label="Commits per day, {since}">
					{#each columns as c (c.day)}
						<div class="col" title={columnLabel(c)}>
							<div class="bar" style="height: {(c.commits / maxCommits) * 100}%"></div>
						</div>
					{/each}
				</div>
			{/if}
			<div class="axis small muted"><span>{utcDay(columns[0].day)}</span><span>{utcDay(columns[columns.length - 1].day)}</span></div>
			<details class="small">
				<summary>As a table</summary>
				<table>
					<thead><tr><th>Day (UTC)</th><th>Hackatime</th>{#if github}<th>Commits</th>{/if}</tr></thead>
					<tbody>
						{#each columns.filter((c) => c.seconds || c.commits) as c (c.day)}
							<tr><td>{utcDay(c.day)}</td><td>{hours(c.seconds)}</td>{#if github}<td>{c.commits}</td>{/if}</tr>
						{/each}
					</tbody>
				</table>
			</details>
		</div>
	{/if}

	{#if github?.commits.length}
		<div class="stack-sm">
			<h3 class="small">Commits {since}</h3>
			<ul class="commits small">
				{#each commits as c (c.sha)}
					<li>
						<a href="{github.url}/commit/{c.sha}" target="_blank" rel="noreferrer" class="sha">{c.sha.slice(0, 7)}</a>
						<span class="message">{c.message}</span>
						{#if c.has_lines}<span class="lines"><span class="add">+{c.additions}</span> <span class="del">−{c.deletions}</span></span>{/if}
						<span class="muted">{c.author} · {shortDate(c.date)}</span>
					</li>
				{/each}
			</ul>
			{#if github.commits.length > 8}
				<button class="link-btn small" onclick={() => (showAll = !showAll)}>{showAll ? 'Show fewer' : `Show all ${github.commits.length}`}</button>
			{/if}
		</div>
	{/if}
</section>

<style>
	.facts { gap: 6px 10px; }
	.tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; }
	.tile { display: flex; flex-direction: column; gap: 2px; padding: 10px 14px; border-radius: 10px; background: var(--navy); }
	.tile .label { font-size: 18px; color: var(--muted); }
	.tile strong { font-size: 24px; color: var(--text); }
	.tile .authors { font-size: 18px; overflow-wrap: anywhere; }
	.strip-head { display: flex; justify-content: space-between; gap: 8px; }
	.strip { display: flex; align-items: flex-end; gap: 2px; height: 90px; padding-bottom: 1px; border-bottom: 1px solid var(--deep); }
	.strip.short { height: 50px; }
	.col { flex: 1; height: 100%; display: flex; align-items: flex-end; min-width: 3px; }
	.col:hover { background: rgb(255 255 255 / 0.06); }
	.bar { width: 100%; min-height: 0; background: var(--info); border-radius: 4px 4px 0 0; }
	.axis { display: flex; justify-content: space-between; }
	table { width: 100%; border-collapse: collapse; margin-top: 6px; }
	th, td { text-align: left; padding: 4px 8px; border-bottom: 1px solid var(--deep); }
	.commits { list-style: none; padding: 0; display: flex; flex-direction: column; gap: 6px; }
	.commits li { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 10px; padding: 6px 10px; border-radius: 8px; background: rgb(0 0 0 / 0.25); }
	.sha { font-family: ui-monospace, monospace; }
	.message { flex: 1 1 200px; overflow-wrap: anywhere; }
	.add { color: var(--good); }
	.del { color: var(--danger); }
</style>
