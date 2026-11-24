<script>
	// On a rejected ship: soft or hard, and for HQ and admins, what can be
	// done about it. Soft: they change the project and ship it again. Hard:
	// the project can't be shipped again until HQ or an admin allows it, and
	// its tickets are taken back; allowing it can give them back. Undoing puts
	// the ship back in the review queue.
	import { api } from '#lib/api.js';
	import { session } from '#lib/session.svelte.js';
	import { date } from '#lib/format.js';
	import { canManageRejections } from '#lib/review.js';

	// block: the ship page's resubmit_block, or null.
	let { submissionId, projectId, block, mine = false, onchange } = $props();

	const manage = $derived(canManageRejections(session.me) && !mine);
	let asking = $state(null); // 'block' or 'undo' while the reason box is open
	let reason = $state('');
	let restore = $state(false); // give back what the hard reject took
	let busy = $state(false);
	let error = $state('');

	async function run(fn) {
		busy = true;
		error = '';
		try {
			await fn();
			asking = null;
			reason = '';
			await onchange?.();
		} catch (e) {
			error = e.message;
		} finally {
			busy = false;
		}
	}
	const restoring = $derived(restore && block?.forfeited_stamps > 0);
	const setBlocked = (blocked) =>
		run(() => api('PUT', `/review/projects/${projectId}/resubmit-block`, blocked ? { blocked, reason } : { blocked, restore_stamps: restoring }));
	const undo = () => run(() => api('POST', `/review/submissions/${submissionId}/reopen`, { reason, restore_stamps: restoring }));
</script>

<div class="stack-sm">
	{#if block}
		<p class="callout warn small">
			<strong>Hard rejected{block.by ? ` by ${block.by}` : ''}, {date(block.at)}.</strong> The project can't be shipped again.
			{#if block.forfeited_stamps}The {block.forfeited_stamps} tickets it earned were taken back.{/if}
			{#if block.reason}<span class="pre">{block.reason}</span>{/if}
		</p>
	{:else}
		<p class="callout info small"><strong>Soft rejected.</strong> They can change the project and ship it again.</p>
	{/if}

	{#if manage}
		<!-- Shown with the undo form too, so it's what they confirm. -->
		{#if block?.forfeited_stamps > 0}
			<label class="check small"><input type="checkbox" bind:checked={restore} /> Give back the {block.forfeited_stamps} tickets it forfeited</label>
		{/if}
		{#if asking}
			<form class="stack-sm" onsubmit={(e) => { e.preventDefault(); asking === 'undo' ? undo() : setBlocked(true); }}>
				<label class="field">
					<span>{asking === 'undo' ? 'Why undo it? For the reviewer who picks it up' : "Why can't it be shipped again? For HQ and the admins"}</span>
					<!-- svelte-ignore a11y_autofocus -->
					<textarea bind:value={reason} required autofocus style="min-height: 64px"></textarea>
				</label>
				<div class="row">
					<button class="btn small primary" disabled={busy || !reason.trim()}>{asking === 'undo' ? 'Put it back in review' : "Stop it being shipped again"}</button>
					<button type="button" class="btn small ghost" onclick={() => (asking = null)}>Cancel</button>
				</div>
			</form>
		{:else}
			<div class="row">
				{#if block}
					<button class="btn small" disabled={busy} onclick={() => setBlocked(false)}>Let them ship it again</button>
				{:else}
					<button class="btn small ghost" disabled={busy} onclick={() => (asking = 'block')}>Make it a hard reject</button>
				{/if}
				<button class="btn small ghost" disabled={busy} onclick={() => (asking = 'undo')}>Undo the rejection</button>
			</div>
			<p class="small muted">Undoing puts this ship back in the review queue and lets the project be shipped again. It doesn't count as a review.</p>
		{/if}
	{/if}
	{#if error}<p class="callout warn small" role="alert">{error}</p>{/if}
</div>

<style>
	.pre { display: block; margin-top: 4px; white-space: pre-wrap; }
</style>
