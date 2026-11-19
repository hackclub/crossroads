<script>
	// Asks everyone once whether they want to ride the trains on the landing
	// page (on_landing_page is null until they answer). Closing it without
	// answering puts it off until they next open the platform. Settings can
	// change the answer later.
	import { api } from './api.js';
	import { session, refresh } from './session.svelte.js';

	let dialog;
	let closed = $state(false);
	let saving = $state(false);
	let error = $state('');

	$effect(() => {
		if (session.me?.on_landing_page === null && !closed && !dialog.open) dialog.showModal();
	});

	async function answer(yes) {
		error = '';
		saving = true;
		try {
			await api('PATCH', '/me', { on_landing_page: yes });
			await refresh();
			dialog.close();
		} catch (e) {
			error = e.message;
		} finally {
			saving = false;
		}
	}
</script>

<dialog class="prompt" bind:this={dialog} onclose={() => (closed = true)} aria-labelledby="prompt-title">
	<div class="stack">
		<div class="row head">
			{#if session.me?.slack_id}
				<div class="car" aria-hidden="true"><img src="https://cachet.dunkirk.sh/users/{session.me.slack_id}/r" alt="" /></div>
			{/if}
			<h2 id="prompt-title">Want to ride the train?</h2>
		</div>
		<p>
			The trains on the Crossroads homepage carry people's Slack pictures. Want yours on one? It shows once you've had a project approved: just the
			picture, not your name.
		</p>
		<p class="small muted">You can change your mind any time in <a href="/settings" onclick={() => dialog.close()}>Settings</a>.</p>
		{#if error}<p class="callout warn small" role="alert">{error}</p>{/if}
		<div class="row actions">
			<span class="spacer"></span>
			<!-- Focus starts on no, so Enter doesn't put anyone on the homepage. -->
			<!-- svelte-ignore a11y_autofocus -->
			<button type="button" class="btn ghost" disabled={saving} onclick={() => answer(false)} autofocus>No thanks</button>
			<button type="button" class="btn primary" disabled={saving} onclick={() => answer(true)}>Yes, put me on</button>
		</div>
	</div>
</dialog>

<style>
	.prompt {
		width: min(560px, calc(100vw - 32px));
		padding: 24px 28px;
		color: var(--text);
		background: var(--row-bg);
		border: 0;
		border-radius: var(--radius);
		box-shadow: 0 24px 60px rgb(0 0 0 / 0.6);
		font-family: 'Crimson Pro', Georgia, serif;
	}
	.prompt::backdrop { background: rgb(0 0 0 / 0.7); }
	.prompt h2 { font-size: 36px; }
	.prompt p { margin: 0; }
	.prompt .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); }
	.head { flex-wrap: nowrap; gap: 16px; }
	/* A car from the landing page's trains, seen from above, with them in its window. */
	.car {
		display: grid;
		place-items: center;
		flex: none;
		width: 50px;
		height: 70px;
		background: #0a0a0a;
		border: 1px solid rgb(255 255 255 / 0.06);
	}
	.car img { display: block; width: 35px; height: 35px; object-fit: cover; }
	.actions { align-items: center; }
	.spacer { flex: 1; }
</style>
