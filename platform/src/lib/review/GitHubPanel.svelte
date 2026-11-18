<script>
	// The repo on GitHub and its commits during this ship (backend/api/checks.go).
	import { utcDay } from '#lib/format.js';
	import { dayColumns, shortDate } from './days.js';
	import DayChart from './DayChart.svelte';

	// commitSeconds: Hackatime's time since the commit before, by SHA (like
	// Ari); afterLastCommit: tracked after the last commit, up to shipping.
	let { github, githubError, repoUrl, days, windowStart, reship, commitSeconds = null, afterLastCommit = null, commitTimeError = '' } = $props();
	// "2h 13m", "45m", "0m".
	const span = (s) => {
		const m = Math.round(s / 60);
		return m >= 60 ? `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, '0')}m` : `${m}m`;
	};
	// A long stretch of tracked time behind a single commit is worth a look.
	const LONG = 3 * 3600;
	const since = $derived(reship ? 'since the last approved ship' : 'since Crossroads started');
	const columns = $derived(dayColumns(days, windowStart));
	const onGitHub = $derived(/^https?:\/\/(www\.)?github\.com\/[^/]+\/[^/]+/i.test(repoUrl ?? ''));
</script>

{#if github}
	<div class="tab-grid">
		<div class="left stack">
			<div class="row between">
				<a href={github.url} target="_blank" rel="noreferrer">{github.owner}/{github.repo}</a>
				<span class="row facts small">
					<span class="chip">Public</span>
					{#if github.language}<span class="chip">{github.language}</span>{/if}
					{#if github.fork}<span class="badge warn">Fork</span>{/if}
					{#if github.archived}<span class="badge">Archived</span>{/if}
				</span>
			</div>
			<p class="small muted">Created {shortDate(github.created_at)} · last push {shortDate(github.pushed_at)} · {github.total_commits} commit{github.total_commits === 1 ? '' : 's'} all time{github.stars ? ` · ${github.stars} stars` : ''}</p>
			<div class="tiles">
				<div class="tile"><span class="label">Commits {since}</span><strong>{github.commits.length}{github.truncated ? '+' : ''}</strong></div>
				<div class="tile">
					<span class="label">Lines changed</span>
					{#if github.lines_counted}
						<strong>+{github.additions.toLocaleString()} <span class="muted">/ −{github.deletions.toLocaleString()}</span></strong>
					{:else}
						<strong class="muted">–</strong>
					{/if}
				</div>
				<div class="tile wide">
					<span class="label">By</span>
					<strong class="authors">{#each github.authors as a, i}{i ? ', ' : ''}{a.author} <span class="muted">({a.commits})</span>{:else}<span class="muted">nobody</span>{/each}</strong>
				</div>
			</div>
			<p class="small muted">On the default branch, from {shortDate(github.since)} until it shipped.{github.lines_note ? ' ' + github.lines_note : ''}</p>
			{#if columns.length}
				<DayChart {columns} value={(c) => c.commits} label="Commits per day" height={70}
					summary="{columns.reduce((s, c) => s + c.commits, 0)} in all" format={(v) => `${v} commit${v === 1 ? '' : 's'}`} />
			{/if}
		</div>
		<div class="right stack-sm">
			<h3 class="small">{github.commits.length}{github.truncated ? '+' : ''} commits {since}</h3>
			{#if commitSeconds}
				<p class="small muted">Each commit shows the Hackatime time tracked since the commit before it.{afterLastCommit ? ` ${span(afterLastCommit)} was tracked after the last commit, before shipping.` : ''}</p>
			{:else if commitTimeError}
				<p class="small muted">Couldn't get the time between commits from Hackatime: {commitTimeError}</p>
			{/if}
			{#if github.commits.length}
				<ul class="commits small">
					{#each github.commits as c (c.sha)}
						<li>
							{#if commitSeconds}
								{@const s = commitSeconds[c.sha]}
								<span class="took" class:long={s >= LONG} title={s == null ? "The commit before this one wasn't fetched, so its time isn't known." : `${span(s)} tracked on Hackatime since the commit before`}>{s == null ? '?' : span(s)}</span>
							{/if}
							<a href="{github.url}/commit/{c.sha}" target="_blank" rel="noreferrer" class="sha">{c.sha.slice(0, 7)}</a>
							<span class="message">{c.message}</span>
							{#if c.has_lines}<span><span class="add">+{c.additions}</span> <span class="del">−{c.deletions}</span></span>{/if}
							<span class="muted">{c.author} · {shortDate(c.date)}</span>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="small muted">None: was the work pushed, or done before?</p>
			{/if}
		</div>
	</div>
{:else if githubError}
	<p class="callout warn small">Couldn't ask GitHub: {githubError}</p>
{:else if onGitHub}
	<p class="callout warn small">GitHub says <a href={repoUrl} target="_blank" rel="noreferrer">this repo</a> doesn't exist or is private, so there's nothing to show. Reviewers have to be able to see the code.</p>
{:else if repoUrl}
	<p class="small muted">The repo isn't on GitHub (<a href={repoUrl} target="_blank" rel="noreferrer">{repoUrl}</a>), so there are no commit stats. Check it by hand.</p>
{:else}
	<p class="small muted">There's no repo link.</p>
{/if}

<style>
	.tab-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px 28px; container-type: inline-size; }
	@media (min-width: 1300px) { .tab-grid { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); align-items: start; } }
	.facts { gap: 6px; }
	.facts .chip { font-size: 15px; padding: 0 10px; }
	.left p { margin: 0; }
	.tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 8px; }
	.tile { display: flex; flex-direction: column; padding: 6px 12px; border-radius: 10px; background: var(--navy); }
	.tile .label { font-size: 15px; color: var(--muted); }
	.tile strong { font-size: 22px; font-weight: 400; font-variant-numeric: tabular-nums; }
	.tile .authors { font-size: 17px; overflow-wrap: anywhere; }
	.commits { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 4px; }
	.commits li { display: flex; flex-wrap: wrap; align-items: baseline; gap: 2px 10px; padding: 4px 10px; border-radius: 8px; background: var(--rv-sunk, rgb(0 0 0 / 0.25)); }
	.sha { font-family: ui-monospace, monospace; }
	.took { min-width: 4.6em; font-variant-numeric: tabular-nums; font-weight: 600; }
	.took.long { color: var(--warn); }
	.message { flex: 1 1 200px; overflow-wrap: anywhere; }
	.add { color: var(--good); }
	.del { color: var(--danger); }
</style>
