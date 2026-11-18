<script>
	// The person's history in the Unified YSWS DB: their trust level and
	// weighted grants from YSWS Authors, and every project another program
	// approved for them (backend/unified, FindByPerson).
	let { person, error = '' } = $props();

	const SHOWN = 5;
	let all = $state(false);
	const projects = $derived(person?.projects ?? []);
	const shown = $derived(all ? projects : projects.slice(0, SHOWN));
	const totalHours = $derived(projects.reduce((s, p) => s + (p.hours || 0), 0));
	const round = (n) => Math.round(n * 10) / 10;
	const when = (d) => (d ? new Date(d + 'T12:00:00').toLocaleDateString(undefined, { month: 'short', year: 'numeric' }) : '');
	const repo = (url) => url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '');
</script>

<div class="stack-sm history">
	<h3 class="small">In the Unified YSWS DB</h3>
	{#if error}
		<p class="small muted">{error}</p>
	{:else if !person}
		<p class="small muted">Not searched: no Unified YSWS DB token is set.</p>
	{:else if !person.found && !projects.length}
		<p class="small muted">Nothing: no projects approved by other programs yet.</p>
	{:else}
		<p class="small">
			{#if person.trust_level}<span class="badge info tiny">{person.trust_level}</span>{/if}
			{projects.length} project{projects.length === 1 ? '' : 's'}, {round(totalHours)}h{person.found ? ` · ${round(person.weighted_grants)} weighted grants` : ''}
		</p>
		<ul class="past small">
			{#each shown as p, i (i)}
				<li>
					<div class="row between">
						<strong>{p.program || 'Unknown program'}</strong>
						<span class="muted">{round(p.hours)}h{p.approved_at ? ` · ${when(p.approved_at)}` : ''}</span>
					</div>
					{#if p.description}<span class="desc">{p.description}</span>{/if}
					<span class="links">
						{#if p.code_url}<a href={p.code_url} target="_blank" rel="noreferrer">{repo(p.code_url)}</a>{/if}
						{#if p.playable_url && p.playable_url !== p.code_url}<a href={p.playable_url} target="_blank" rel="noreferrer">Demo</a>{/if}
					</span>
				</li>
			{/each}
		</ul>
		{#if projects.length > SHOWN}
			<button class="link-btn small" onclick={() => (all = !all)}>{all ? 'Show fewer' : `Show all ${projects.length}`}</button>
		{/if}
	{/if}
</div>

<style>
	.history { margin-top: 12px; }
	.history p { margin: 0; }
	.past { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 4px; }
	.past li { display: flex; flex-direction: column; gap: 2px; padding: 6px 10px; border-radius: 8px; background: var(--rv-sunk, rgb(0 0 0 / 0.25)); min-width: 0; }
	.desc { color: var(--muted); overflow-wrap: anywhere; }
	.links { display: flex; flex-wrap: wrap; gap: 4px 12px; min-width: 0; }
	.links a { overflow-wrap: anywhere; }
	.tiny { font-size: 14px; padding: 0 8px; }
</style>
