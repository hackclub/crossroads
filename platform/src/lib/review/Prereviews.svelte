<script>
	// AI pre-reviews without a server running Claude (backend/api/prereview.go):
	// copy one prompt with every ship that doesn't have one, give it to your
	// own Claude, and paste the answer back. Or run scripts/prereview.mjs on a
	// laptop with Claude Code, which reads each repo itself.
	import { api, get } from '#lib/api.js';

	const SIZES = [5, 10, 20];
	let size = $state(10);
	let redo = $state(false); // ships that already have one too
	let prompt = $state(null); // { prompt, ship_ids, waiting }
	let copied = $state(false);
	let making = $state(false);
	let answer = $state('');
	let saving = $state(false);
	let result = $state(null); // { saved, failed }
	let error = $state('');

	async function makePrompt() {
		making = true;
		error = '';
		copied = false;
		try {
			prompt = await get(`/review/prereviews/prompt?limit=${size}${redo ? '&all=1' : ''}`);
			if (prompt.ship_ids.length) {
				try {
					await navigator.clipboard.writeText(prompt.prompt);
					copied = true;
				} catch {
					copied = false; // the box below has it to copy by hand
				}
			}
		} catch (e) {
			error = e.message;
		} finally {
			making = false;
		}
	}

	async function downloadBatch() {
		error = '';
		try {
			const batch = await get(`/review/prereviews/batch?limit=${size}${redo ? '&all=1' : ''}`);
			const url = URL.createObjectURL(new Blob([JSON.stringify(batch, null, 2)], { type: 'application/json' }));
			const a = Object.assign(document.createElement('a'), { href: url, download: 'crossroads-prereviews.json' });
			a.click();
			URL.revokeObjectURL(url);
		} catch (e) {
			error = e.message;
		}
	}

	async function readFile(e) {
		const file = e.currentTarget.files?.[0];
		if (file) answer = await file.text();
	}

	async function save(e) {
		e.preventDefault();
		saving = true;
		error = '';
		result = null;
		try {
			// The script's answer file says it's from the script and which model.
			let body = { text: answer };
			try {
				const parsed = JSON.parse(answer);
				if (parsed?.source === 'script' && Array.isArray(parsed.prereviews)) body = { prereviews: parsed.prereviews, model: parsed.model ?? '', source: 'script' };
			} catch {
				// Claude's answer as pasted: the site finds the JSON in it.
			}
			result = await api('POST', '/review/prereviews', body);
			if (!result.failed.length) answer = '';
		} catch (err) {
			error = err.message;
		} finally {
			saving = false;
		}
	}
</script>

<div class="stack ai">
	<p class="small muted">Claude reads each ship first and writes what it is, how big it is, how AI-written it looks and what to check. Reviewers see it in the ship's AI pre-review tab. It decides nothing and isn't a review.</p>

	<section class="card stack-sm">
		<h2>1. Copy the prompt</h2>
		<div class="row wrap">
			<label class="inline small">Ships <select bind:value={size}>{#each SIZES as n (n)}<option value={n}>{n}</option>{/each}</select></label>
			<label class="inline small"><input type="checkbox" bind:checked={redo} /> Redo ones that have one</label>
			<button class="btn small primary" disabled={making} onclick={makePrompt}>{making ? 'Gathering the ships…' : 'Copy the prompt'}</button>
		</div>
		{#if prompt}
			{#if !prompt.ship_ids.length}
				<p class="small">Every ship in review has a pre-review.</p>
			{:else}
				<p class="small" role="status">
					{copied ? 'Copied' : 'Made'} a prompt for {prompt.ship_ids.length} ship{prompt.ship_ids.length === 1 ? '' : 's'}{prompt.waiting > prompt.ship_ids.length ? `, of ${prompt.waiting} without one` : ''}.
					Paste it into Claude (claude.ai with web search on lets it open the repos and demos).
				</p>
				<details class="small">
					<summary>{copied ? 'See the prompt' : 'Copy it from here'}</summary>
					<textarea readonly value={prompt.prompt} onfocus={(e) => e.currentTarget.select()}></textarea>
				</details>
			{/if}
		{/if}
	</section>

	<form class="card stack-sm" onsubmit={save}>
		<h2>2. Paste Claude's answer</h2>
		<textarea bind:value={answer} placeholder={'{"prereviews": [{"ship_id": …}]}'} aria-label="Claude's answer"></textarea>
		<div class="row wrap">
			<button class="btn small primary" disabled={saving || !answer.trim()}>{saving ? 'Saving…' : 'Save the pre-reviews'}</button>
			<label class="inline small">or the script's answer file <input type="file" accept=".json,application/json" onchange={readFile} /></label>
		</div>
		{#if result}
			<div class="small stack-sm" role="status">
				{#if result.saved.length}
					<p class="callout good small"><span>Saved {result.saved.length}:
						{#each result.saved as id, i (id)}{i ? ', ' : ''}<a href="/review/{id}">ship {id}</a>{/each}.</span></p>
				{/if}
				{#if result.failed.length}
					<div class="callout warn small">
						<p>Not saved:</p>
						<ul>{#each result.failed as f, i (i)}<li><a href="/review/{f.ship_id}">ship {f.ship_id}</a>: {f.error}</li>{/each}</ul>
					</div>
				{/if}
			</div>
		{/if}
	</form>

	{#if error}<p class="callout warn small" role="alert">{error}</p>{/if}

	<details class="card small">
		<summary>Or run it from a laptop with Claude Code</summary>
		<div class="stack-sm">
			<p>The script clones each repo and runs your signed-in Claude Code on it, so it reads the actual code and git history. It uses your Claude subscription, not a server.</p>
			<ol>
				<li><button class="link-btn" type="button" onclick={downloadBatch}>Download the ships</button> (the size and redo choices above apply).</li>
				<li>In the Crossroads repo: <code>node scripts/prereview.mjs --from crossroads-prereviews.json --out answer.json</code></li>
				<li>Choose <code>answer.json</code> above, under step 2, and save.</li>
			</ol>
			<p class="muted">If the site has <code>PREREVIEW_TOKEN</code> set, <code>PREREVIEW_TOKEN=… node scripts/prereview.mjs --base https://…</code> fetches and uploads by itself.</p>
		</div>
	</details>
</div>

<style>
	.ai { max-width: 860px; }
	.ai h2 { font-size: 24px; margin: 0; }
	.ai p { margin: 0; }
	.wrap { flex-wrap: wrap; }
	textarea { width: 100%; min-height: 140px; font-family: ui-monospace, monospace; font-size: 13px; }
	details textarea { margin-top: 8px; min-height: 220px; }
	.inline { display: inline-flex; flex-wrap: wrap; align-items: center; gap: 8px; margin: 0; max-width: 100%; }
	.inline select { width: auto; }
	.inline input[type='file'] { font-size: 14px; width: auto; max-width: 100%; }
	code { font-size: 0.75em; overflow-wrap: anywhere; }
	.wrap { align-items: center; gap: 12px 20px; }
	ol, ul { margin: 0; padding-left: 1.4em; display: flex; flex-direction: column; gap: 4px; }
	summary { cursor: pointer; }
	details > div { margin-top: 10px; }
</style>
