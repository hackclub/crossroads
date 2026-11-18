<script>
	// On the review page: fix a project's demo link or screenshot instead of
	// rejecting it for a wrong one. The maker sees a note on their project
	// saying what changed.
	import { api } from '#lib/api.js';
	import ImageDrop from '#lib/ImageDrop.svelte';

	let { project, onsaved } = $props();

	let dialog;
	let demo = $state('');
	let shot = $state('');
	let busy = $state(false);
	let error = $state('');

	function start() {
		demo = project.demo_url ?? '';
		shot = project.screenshot_url ?? '';
		error = '';
		dialog.showModal();
	}

	async function save(e) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			await api('PATCH', `/review/projects/${project.id}`, { demo_url: demo, screenshot_url: shot });
			dialog.close();
			await onsaved?.();
		} catch (err) {
			error = err.message;
		} finally {
			busy = false;
		}
	}
</script>

<button class="link-btn small" onclick={start} title="Change the demo link or screenshot instead of rejecting for it">Fix links</button>
<dialog class="fix-dialog" bind:this={dialog} aria-labelledby="fix-title">
	<form class="fix stack-sm" onsubmit={save}>
		<h2 id="fix-title">Fix {project.name}'s links</h2>
		<label class="field">
			<span>Demo link</span>
			<input type="url" bind:value={demo} required placeholder="https://" />
			<small>Something they can try: a site, a release, itch.io.</small>
		</label>
		<div class="field">
			<span>Screenshot</span>
			<ImageDrop bind:value={shot} id="fix-screenshot" label="Drop a screenshot of it working" />
			<input type="url" bind:value={shot} required placeholder="or a link: https://" aria-label="Screenshot link" />
		</div>
		<p class="small muted">They see a note on their project saying what you changed.</p>
		{#if error}<p class="callout warn small" role="alert">{error}</p>{/if}
		<div class="row">
			<button class="btn small primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
			<button type="button" class="btn small ghost" onclick={() => dialog.close()}>Cancel</button>
		</div>
	</form>
</dialog>

<style>
	.fix-dialog {
		width: min(520px, calc(100vw - 32px));
		padding: 24px 28px;
		color: var(--text);
		background: var(--row-bg);
		border: 0;
		border-radius: var(--radius);
		box-shadow: 0 24px 60px rgb(0 0 0 / 0.6);
		font-family: 'Crimson Pro', Georgia, serif;
	}
	.fix-dialog::backdrop { background: rgb(0 0 0 / 0.7); }
	.fix-dialog h2 { font-size: 30px; margin: 0; }
	.fix-dialog p { margin: 0; }
</style>
