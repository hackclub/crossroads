<script>
	// A ship's pre-review: what Claude made of it before anyone looked,
	// pasted in from the review queue's Pre-reviews tab or uploaded by
	// scripts/prereview.mjs (backend/api/prereview.go). It decides nothing.
	import { date } from '#lib/format.js';

	let { prereview } = $props();
	const LEVELS = {
		none: ['good', 'No signs of AI'],
		low: ['info', 'A little AI'],
		medium: ['warn', 'Some AI'],
		high: ['warn', 'Mostly AI'],
		unknown: ['info', 'AI use not rated']
	};
	const level = $derived(LEVELS[prereview?.ai_usage?.level] ?? LEVELS.unknown);
</script>

{#if !prereview}
	<p class="small muted">No pre-review for this ship yet. Make them on the Review page, in the <a href="/review?stage=prereviews">Pre-reviews</a> tab.</p>
{:else}
	<div class="tab-grid">
		<div class="left stack">
			<p class="small muted">
				Claude's read, {prereview.source === 'script' ? 'from the script that cloned the repo' : `pasted${prereview.by ? ` by ${prereview.by}` : ''}`},
				{date(prereview.created_at)}{prereview.model ? ` (${prereview.model})` : ''}. It can be wrong: check what matters yourself.
			</p>
			{#if prereview.summary}<p class="summary">{prereview.summary}</p>{/if}
			{#if prereview.scale}<p class="small"><strong>Size:</strong> {prereview.scale}</p>{/if}
			{#if prereview.features?.length}
				<div class="stack-sm">
					<h3 class="small">What it does</h3>
					<ul class="small">{#each prereview.features as f, i (i)}<li>{f}</li>{/each}</ul>
				</div>
			{/if}
			{#if prereview.readme || prereview.demo}
				<dl class="small facts">
					{#if prereview.readme}<dt>README</dt><dd>{prereview.readme}</dd>{/if}
					{#if prereview.demo}<dt>Demo</dt><dd>{prereview.demo}</dd>{/if}
				</dl>
			{/if}
		</div>
		<div class="right stack">
			<div class="stack-sm">
				<h3 class="small">AI use <span class="badge {level[0]} tiny">{level[1]}</span></h3>
				{#if prereview.ai_usage?.evidence?.length}
					<ul class="small evidence">{#each prereview.ai_usage.evidence as e, i (i)}<li>{e}</li>{/each}</ul>
				{/if}
			</div>
			{#if prereview.notes?.length}
				<div class="stack-sm">
					<h3 class="small">Worth knowing</h3>
					<ul class="small notes">{#each prereview.notes as n, i (i)}<li>{n}</li>{/each}</ul>
				</div>
			{/if}
			{#if prereview.contributions?.length}
				<div class="stack-sm">
					<h3 class="small">Who made what</h3>
					<ul class="small people">
						{#each prereview.contributions as c, i (i)}
							<li><strong>{c.name}</strong> <span class="muted">{c.commits} commit{c.commits === 1 ? '' : 's'}</span>{#if c.areas} · {c.areas}{/if}</li>
						{/each}
					</ul>
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.tab-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 20px 28px; }
	@media (min-width: 1300px) { .tab-grid { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); align-items: start; } }
	p { margin: 0; }
	.summary { font-size: 20px; line-height: 1.35; }
	h3 { margin: 0; display: flex; align-items: center; gap: 8px; }
	ul { margin: 0; padding-left: 1.2em; display: flex; flex-direction: column; gap: 2px; }
	.people { list-style: none; padding: 0; }
	.evidence li, .notes li { overflow-wrap: anywhere; }
	.facts { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 4px 12px; margin: 0; }
	.facts dt { color: var(--muted); }
	.facts dd { margin: 0; }
	.tiny { font-size: 14px; padding: 0 8px; }
</style>
