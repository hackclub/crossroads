<script>
	// The screenshot block on the project form: drop an image on it, click to
	// choose one, or paste one while it's focused. Uploads to /api/uploads and
	// sets `value` to the image's URL. A link can still be typed below it.
	import { uploadImage } from './api.js';

	let { value = $bindable(''), id = 'screenshot', disabled = false, label = 'Drop a screenshot here' } = $props();

	const MAX = 5 * 1024 * 1024;
	const TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp'];

	let input;
	let over = $state(false);
	let busy = $state(false);
	let error = $state('');
	let broken = $state(false);

	async function take(file) {
		if (!file || disabled) return;
		error = '';
		if (!TYPES.includes(file.type)) return (error = 'Use a PNG, JPEG, GIF or WebP image.');
		if (file.size > MAX) return (error = 'That image is too big. The limit is 5 MB.');
		busy = true;
		try {
			value = await uploadImage(file);
			broken = false;
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}

	function drop(e) {
		e.preventDefault();
		over = false;
		take(e.dataTransfer?.files?.[0]);
	}

	function paste(e) {
		const file = [...(e.clipboardData?.items ?? [])].find((i) => i.kind === 'file')?.getAsFile();
		if (file) {
			e.preventDefault();
			take(file);
		}
	}
</script>

<div class="drop-wrap">
	<button
		type="button"
		class="screenshot-well drop"
		class:over
		class:busy
		{disabled}
		aria-describedby="{id}-help"
		onclick={() => input.click()}
		ondragenter={(e) => { e.preventDefault(); over = true; }}
		ondragover={(e) => e.preventDefault()}
		ondragleave={() => (over = false)}
		ondrop={drop}
		onpaste={paste}
	>
		{#if value && !broken}
			<img src={value} alt="Screenshot" onerror={() => (broken = true)} />
		{:else}
			<span class="prompt">
				<span class="big-text">{busy ? 'Uploading…' : over ? 'Drop it!' : label}</span>
				<span class="small muted">{broken ? "That link didn't load as an image. " : ''}or click to choose one, or paste it. PNG, JPEG, GIF or WebP, up to 5 MB.</span>
			</span>
		{/if}
		{#if busy && value}<span class="overlay">Uploading…</span>{/if}
	</button>
	<input bind:this={input} type="file" accept={TYPES.join(',')} hidden onchange={(e) => { take(e.currentTarget.files[0]); e.currentTarget.value = ''; }} />

	{#if value && !busy}
		<div class="row actions">
			<button type="button" class="chip" {disabled} onclick={() => input.click()}>Replace</button>
			<button type="button" class="chip" {disabled} onclick={() => { value = ''; broken = false; }}>Remove</button>
		</div>
	{/if}
</div>
{#if error}<p class="callout warn small" role="alert">{error}</p>{/if}

<style>
	.drop-wrap { position: relative; }
	.drop {
		width: 100%;
		border: 0;
		color: var(--text);
		font: inherit;
		cursor: pointer;
		outline-offset: -4px;
		transition: background-color 120ms ease-out;
	}
	.drop:hover, .drop.over { background: #1c2238; }
	.drop.over { outline: 3px dashed var(--head); }
	.drop.busy { cursor: progress; }
	.drop:disabled { cursor: not-allowed; }
	.prompt { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 32px 24px; text-align: center; }
	.big-text { font-size: 36px; }
	.overlay { position: absolute; inset: 0; display: grid; place-items: center; font-size: 36px; background: rgb(0 0 0 / 0.5); }
	.actions { position: absolute; right: 14px; bottom: 14px; }
	@media (prefers-reduced-motion: reduce) { .drop { transition: none; } }
</style>
