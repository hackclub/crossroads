<script>
	// Feedback templates: snippets reviewers drop into feedback. Shared by
	// everyone who reviews; anyone can change them.
	import { onMount } from 'svelte';
	import { api, get } from '#lib/api.js';

	let templates = $state(null);
	let error = $state('');
	let msg = $state(null);
	let draft = $state({ title: '', body: '' }); // a new one
	let editing = $state(null); // { id, title, body } being edited

	async function load() {
		try {
			templates = await get('/review/templates');
		} catch (e) {
			error = e.message;
		}
	}
	onMount(load);

	async function run(fn, ok) {
		msg = null;
		try {
			await fn();
			msg = { tone: 'good', text: ok };
			await load();
			return true;
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
			return false;
		}
	}

	async function create(e) {
		e.preventDefault();
		if (await run(() => api('POST', '/review/templates', draft), `Added “${draft.title}”.`)) draft = { title: '', body: '' };
	}
	async function save(e) {
		e.preventDefault();
		const { id, title, body } = editing;
		if (await run(() => api('PATCH', `/review/templates/${id}`, { title, body }), `Saved “${title}”.`)) editing = null;
	}
	function remove(t) {
		if (!confirm(`Delete the template “${t.title}”? This is for everyone who reviews.`)) return;
		run(() => api('DELETE', `/review/templates/${t.id}`), `Deleted “${t.title}”.`);
	}
</script>

<div class="crumbs"><a href="/review">Review</a> / Feedback templates</div>
<div class="page-head">
	<div class="stack-sm">
		<h1>Feedback templates</h1>
		<p class="muted">Text you can drop into feedback on the review page. Everyone who reviews shares these. Write them for teenagers, often reading in their second language: short sentences, no idioms.</p>
	</div>
</div>

<div class="stack">
	{#if error}<p class="callout warn">{error}</p>{/if}
	<div role="status" aria-live="polite">{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}</div>

	<div class="grid-2">
		<div class="stack">
			{#each templates ?? [] as t (t.id)}
				<section class="card">
					{#if editing?.id === t.id}
						<form class="stack" onsubmit={save}>
							<label class="field"><span>Title</span><input bind:value={editing.title} required maxlength="100" /></label>
							<label class="field"><span>Text</span><textarea bind:value={editing.body} required></textarea></label>
							<div class="row">
								<button class="btn small primary">Save</button>
								<button type="button" class="btn small ghost" onclick={() => (editing = null)}>Cancel</button>
							</div>
						</form>
					{:else}
						<div class="card-title">
							<h2>{t.title}</h2>
							<div class="row">
								<button class="link-btn small" onclick={() => (editing = { ...t })}>Edit</button>
								<button class="link-btn small" onclick={() => remove(t)}>Delete</button>
							</div>
						</div>
						<p class="small" style="white-space: pre-wrap">{t.body}</p>
					{/if}
				</section>
			{:else}
				{#if templates}<div class="card dashed"><p class="muted">No templates yet.</p></div>{/if}
			{/each}
		</div>

		<form class="card" onsubmit={create}>
			<h2>New template</h2>
			<label class="field"><span>Title</span><input bind:value={draft.title} required maxlength="100" placeholder="Demo link is broken" /></label>
			<label class="field"><span>Text</span><textarea bind:value={draft.body} required placeholder="Your demo link doesn't open. Please fix it and ship again."></textarea></label>
			<div class="row"><button class="btn primary">Add</button></div>
		</form>
	</div>
</div>
