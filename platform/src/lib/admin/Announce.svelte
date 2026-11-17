<script>
	// The Announce tab: write a Slack announcement, try it in your DMs, and
	// post it to the station channel (#crossroads-station from the real site,
	// #xr-station-test from every other build), then see how it does.
	// Superadmins only (/api/superadmin/announcements; planning/ROUTES.md).
	// slack.js builds the message; the preview shows exactly what's sent.
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';
	import { date } from '../format.js';
	import { session } from '../session.svelte.js';
	import ImageDrop from '../ImageDrop.svelte';
	import RichText from '../announce/RichText.svelte';
	import SlackPreview from '../announce/SlackPreview.svelte';
	import Engagement from '../announce/Engagement.svelte';
	import { toSlack, emptyDoc, newBlock, newButton, PAGES } from '../announce/slack.js';

	let data = $state(null); // { channel, slack_problem, public_url, announcements }
	let stampTypes = $state([]);
	let msg = $state(null);
	// The announcement open in the editor: a GET /superadmin/announcements/{id}
	// (or { id: null } for a new one), with its doc being edited.
	let open = $state(null);
	let saved = $state(''); // the doc as last saved, to tell when there are changes
	let busy = $state('');
	let postDialog;

	const BLOCKS = [
		['text', 'Text'],
		['heading', 'Heading'],
		['image', 'Image'],
		['buttons', 'Buttons'],
		['divider', 'Divider']
	];
	const TARGETS = [
		['page', 'A Crossroads page'],
		['stamp', 'Claim a special stamp'],
		['url', 'Any link']
	];

	const channelName = $derived(data?.channel.name ? '#' + data.channel.name : 'the station channel');
	const built = $derived(open && data ? toSlack(open.doc, { publicUrl: data.public_url, stampTypes }) : null);
	const changed = $derived(open && JSON.stringify(open.doc) !== saved);
	const posted = $derived(!!open?.posted_at);
	const live = $derived(posted && !open.deleted_at);
	const canSend = $derived(!data?.slack_problem);

	async function load() {
		try {
			[data, stampTypes] = await Promise.all([get('/superadmin/announcements'), get('/admin/special-stamp-types')]);
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	}
	onMount(load);

	function startNew() {
		msg = null;
		open = { id: null, doc: emptyDoc() };
		saved = '';
	}

	async function show(id) {
		msg = null;
		busy = 'load';
		try {
			const a = await get(`/superadmin/announcements/${id}`);
			open = a;
			saved = JSON.stringify(a.doc);
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		} finally {
			busy = '';
		}
	}

	// Saves the editor's content: a new draft, a draft's changes, or an edit
	// to the message in Slack. Returns false if it couldn't.
	async function save() {
		if (live && built.problems.length) {
			msg = { tone: 'warn', text: "Fix what's listed under the editor first: saving edits the message in Slack." };
			return false;
		}
		const body = { doc: $state.snapshot(open.doc), blocks: built.blocks, text: built.text };
		const a = open.id ? await api('PATCH', `/superadmin/announcements/${open.id}`, body) : await api('POST', '/superadmin/announcements', body);
		open = { ...open, ...a, doc: open.doc };
		saved = JSON.stringify(open.doc);
		return true;
	}

	// Runs an action with its button showing it's busy and its result in msg.
	async function act(name, action) {
		msg = null;
		busy = name;
		try {
			await action();
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		} finally {
			busy = '';
		}
	}

	const saveNow = () =>
		act('save', async () => {
			if (await save()) {
				msg = { tone: 'good', text: live ? `Saved, and edited in ${channelName}.` : 'Saved the draft.' };
				await load();
			}
		});

	// What's in the editor, saved or not: saving a posted one would edit it.
	const test = () =>
		act('test', async () => {
			await api('POST', '/superadmin/announcements/test', { blocks: built.blocks, text: built.text });
			msg = { tone: 'good', text: "Sent to your DMs with the Crossroads Announcements app. Clicks there don't count." };
		});

	const post = () =>
		act('post', async () => {
			postDialog.close();
			if ((changed || !open.id) && !(await save())) return;
			await api('POST', `/superadmin/announcements/${open.id}/post`);
			await show(open.id);
			msg = { tone: 'good', text: `Posted in ${channelName}.` };
			load();
		});

	const remove = () => {
		const question = !posted
			? 'Delete this draft? It can\'t be brought back.'
			: `Delete it from ${channelName}? Its numbers stay here, but the message is gone for everyone.`;
		if (!confirm(question)) return;
		act('delete', async () => {
			if (!open.id) return (open = null);
			const a = await api('DELETE', `/superadmin/announcements/${open.id}`);
			if (a) {
				await show(open.id);
				msg = { tone: 'good', text: `Deleted from ${channelName}.` };
			} else open = null;
			load();
		});
	};

	const refresh = () => act('refresh', () => show(open.id));

	function close() {
		if (changed && !confirm('Leave without saving your changes?')) return;
		open = null;
		msg = null;
	}

	function move(i, by) {
		const blocks = open.doc.blocks;
		[blocks[i], blocks[i + by]] = [blocks[i + by], blocks[i]];
	}

	const pagePreset = (button) => PAGES.some(([p]) => p === button.page);
	// Notification text in the list: without the @channel and mrkdwn escapes.
	const plain = (text) => text.replace(/^<!channel> /, '').replaceAll('&lt;', '<').replaceAll('&gt;', '>').replaceAll('&amp;', '&');
</script>

<div class="stack">
	{#if data}
		{#if data.slack_problem}
			<p class="callout warn small">{data.slack_problem}</p>
		{:else}
			<p class="muted small">Posts go to <b>{channelName}</b>{#if data.channel.members} ({data.channel.members} people){/if} under your Slack name and picture, from the Crossroads Announcements app.</p>
		{/if}
	{/if}
	{#if msg}<p class="callout {msg.tone} small" role={msg.tone === 'warn' ? 'alert' : 'status'}>{msg.text}</p>{/if}

	{#if !open}
		<div class="row between">
			<h2>Announcements</h2>
			<button class="btn primary" onclick={startNew}>New announcement</button>
		</div>
		<div class="card table-wrap">
			<table>
				<thead><tr><th>Message</th><th>Status</th><th>Clicked</th><th>Clicks</th><th></th></tr></thead>
				<tbody>
					{#each data?.announcements ?? [] as a}
						<tr>
							<td class="message-cell">{#if a.pings}<span class="badge warn">@channel</span> {/if}{plain(a.text)}</td>
							<td class="small">
								{#if a.deleted_at}Deleted from Slack
								{:else if a.posted_at}Posted {date(a.posted_at)} by {a.posted_by_name}
								{:else}<span class="badge info">Draft</span> by {a.created_by_name}, {date(a.updated_at)}{/if}
							</td>
							<td>{a.posted_at ? a.people : ''}</td>
							<td>{a.posted_at ? a.clicks : ''}</td>
							<td class="row">
								<button class="btn small ghost" disabled={busy === 'load'} onclick={() => show(a.id)}>{a.posted_at ? 'Open' : 'Edit'}</button>
								{#if a.permalink && !a.deleted_at}<a class="small" href={a.permalink} target="_blank" rel="noreferrer">In Slack</a>{/if}
							</td>
						</tr>
					{:else}
						<tr><td colspan="5" class="muted">{data ? 'No announcements yet.' : 'Loading…'}</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	{:else}
		<div class="row between">
			<h2>
				{#if !open.id}New announcement{:else if open.deleted_at}Deleted announcement{:else if posted}Posted announcement{:else}Draft{/if}
			</h2>
			<div class="row">
				{#if open.permalink && !open.deleted_at}<a class="small" href={open.permalink} target="_blank" rel="noreferrer">Open in Slack</a>{/if}
				<button class="btn small ghost" onclick={close}>Back to the list</button>
			</div>
		</div>
		{#if posted}
			<p class="muted small">Posted {date(open.posted_at)}. {#if open.deleted_at}Deleted from Slack {date(open.deleted_at)}; it can't be edited, but its numbers are kept.{:else}Saving edits the message in Slack, everywhere it's seen. Buttons keep their clicks as long as you don't remove them.{/if}</p>
		{/if}

		<div class="composer">
			<div class="stack">
				<fieldset class="stack" disabled={!!open.deleted_at}>
					<legend class="sr-only">Message</legend>
					{#each open.doc.blocks as block, i (block.id)}
						<div class="block card">
							<div class="row between block-head">
								<span class="block-kind">{BLOCKS.find(([t]) => t === block.type)[1]}</span>
								<div class="row block-tools">
									<button type="button" class="chip" disabled={i === 0} aria-label="Move up" onclick={() => move(i, -1)}>↑</button>
									<button type="button" class="chip" disabled={i === open.doc.blocks.length - 1} aria-label="Move down" onclick={() => move(i, 1)}>↓</button>
									<button type="button" class="chip" aria-label="Remove this block" onclick={() => open.doc.blocks.splice(i, 1)}>✕</button>
								</div>
							</div>
							{#if block.type === 'text'}
								<RichText bind:content={block.content} label="Text block {i + 1}" editable={!open.deleted_at} />
								<small class="muted hint">Type <code>:emoji:</code> as in Slack. Enter starts a new line.</small>
							{:else if block.type === 'heading'}
								<label class="field"><span>Heading <em>{block.text.length}/150</em></span><input bind:value={block.text} maxlength="150" /></label>
							{:else if block.type === 'image'}
								<ImageDrop bind:value={block.url} id="image-{block.id}" label="Drop an image here" />
								<div class="grid-halves">
									<label class="field"><span>Or a link to it</span><input type="url" bind:value={block.url} placeholder="https://" /></label>
									<label class="field"><span>Title <em>optional</em></span><input bind:value={block.title} maxlength="200" /></label>
								</div>
								<label class="field"><span>Description <em>for screen readers</em></span><input bind:value={block.alt} maxlength="2000" /></label>
							{:else if block.type === 'divider'}
								<p class="muted small">A line across the message.</p>
							{:else if block.type === 'buttons'}
								{#each block.buttons as button, j (button.id)}
									<div class="button-row">
										<div class="grid-halves">
											<label class="field"><span>Label <em>{button.label.length}/75</em></span><input bind:value={button.label} maxlength="75" placeholder="Open Crossroads" /></label>
											<label class="field"><span>Colour</span>
												<select bind:value={button.style}><option value="">Plain</option><option value="primary">Green</option><option value="danger">Red</option></select>
											</label>
											<label class="field"><span>Goes to</span>
												<select bind:value={button.target}>{#each TARGETS as [value, text]}<option {value}>{text}</option>{/each}</select>
											</label>
											{#if button.target === 'page'}
												<label class="field"><span>Page</span>
													<select value={pagePreset(button) ? button.page : 'other'} onchange={(e) => (button.page = e.currentTarget.value === 'other' ? '' : e.currentTarget.value)}>
														{#each PAGES as [path, text]}<option value={path}>{text} ({path})</option>{/each}
														<option value="other">Another page…</option>
													</select>
												</label>
											{:else if button.target === 'stamp'}
												<label class="field"><span>Stamp</span>
													<select bind:value={button.stampTypeId}>
														<option value={null} disabled>Pick one</option>
														{#each stampTypes as t}<option value={t.id} disabled={!t.claim_code}>{t.name}{t.claim_code ? '' : ' (no claim code)'}</option>{/each}
													</select>
												</label>
											{:else}
												<label class="field"><span>Link</span><input type="url" bind:value={button.url} placeholder="https://" /></label>
											{/if}
										</div>
										{#if button.target === 'page' && !pagePreset(button)}
											<label class="field"><span>Path on Crossroads</span><input bind:value={button.page} placeholder="/projects/12" /></label>
										{/if}
										{#if button.target === 'stamp'}
											<small class="muted">Each person can claim it once, while its supply lasts. Give a stamp type a claim code under Special stamp types.</small>
										{/if}
										<div class="row">
											<button type="button" class="link-btn small" onclick={() => block.buttons.splice(j, 1)}>Remove this button</button>
										</div>
									</div>
								{/each}
								{#if block.buttons.length < 25}
									<div class="row"><button type="button" class="btn small ghost" onclick={() => block.buttons.push(newButton())}>Add a button</button></div>
								{/if}
							{/if}
						</div>
					{/each}
					<div class="row add-blocks">
						<span class="muted small">Add</span>
						{#each BLOCKS as [type, text]}
							<button type="button" class="chip" onclick={() => open.doc.blocks.push(newBlock(type))}>{text}</button>
						{/each}
					</div>

					<div class="card stack options">
						<label class="check">
							<input type="checkbox" bind:checked={open.doc.pingChannel} disabled={posted} />
							<span>Ping <b>@channel</b>: notify everyone in {channelName}{#if data?.channel.members} ({data.channel.members} people){/if}</span>
						</label>
						{#if posted}<small class="muted">An edit can't ping anyone, so this stays as it was posted.</small>{/if}
						<label class="field">
							<span>Notification text <em>optional</em></span>
							<input bind:value={open.doc.notification} maxlength="300" placeholder={built?.text.replace(/^<!channel> /, '')} />
						</label>
						<small class="muted">What notifications and screen readers say. Empty uses the first words.</small>
					</div>
				</fieldset>

				{#if built?.problems.length}
					<div class="callout warn small">
						<p>{posted ? 'Fix these before saving:' : 'Fix these before posting (drafts save without them):'}</p>
						<ul>{#each built.problems as problem}<li>{problem}</li>{/each}</ul>
					</div>
				{/if}

				{#if !open.deleted_at}
					<div class="row actions">
						{#if !posted}
							<button class="btn primary" disabled={!!busy || !canSend || built.problems.length > 0} onclick={() => postDialog.showModal()}>Post to {channelName}…</button>
							<button class="btn ghost" disabled={!!busy || !built.blocks.length || (open.id && !changed)} onclick={saveNow}>{busy === 'save' ? 'Saving…' : 'Save draft'}</button>
						{:else}
							<button class="btn primary" disabled={!!busy || !changed || built.problems.length > 0} onclick={saveNow}>{busy === 'save' ? 'Saving…' : 'Save and edit in Slack'}</button>
						{/if}
						<button class="btn ghost" disabled={!!busy || !canSend || !built.blocks.length} onclick={test}>{busy === 'test' ? 'Sending…' : 'Send test to me'}</button>
						<span class="spacer"></span>
						<button class="btn danger small" disabled={!!busy} onclick={remove}>{posted ? 'Delete from Slack' : open.id ? 'Delete draft' : 'Discard'}</button>
					</div>
				{/if}
			</div>

			<div class="preview">
				<SlackPreview blocks={built?.blocks ?? []} name={session.me?.name ?? 'You'} channel={data?.channel.name ?? ''} />
				<small class="muted">Notification: {built?.text}</small>
			</div>
		</div>

		{#if open.engagement}
			<div class="card"><Engagement engagement={open.engagement} publicUrl={data?.public_url ?? ''} onrefresh={refresh} refreshing={busy === 'refresh'} /></div>
		{/if}
	{/if}
</div>

<dialog class="post-dialog" bind:this={postDialog} aria-labelledby="post-title">
	<div class="stack">
		<h2 id="post-title">Post to {channelName}?</h2>
		<p>It goes out right away under your Slack name{#if open?.doc.pingChannel}, and <b>@channel notifies everyone</b> in the channel{#if data?.channel.members} ({data.channel.members} people){/if}{/if}. You can edit or delete it afterwards, but notifications can't be taken back.</p>
		<div class="row actions">
			<span class="spacer"></span>
			<button type="button" class="btn ghost" onclick={() => postDialog.close()}>Cancel</button>
			<button type="button" class="btn primary" disabled={!!busy} onclick={post}>{open?.doc.pingChannel ? 'Post and ping @channel' : 'Post'}</button>
		</div>
	</div>
</dialog>

<style>
	h2 { margin: 0; }
	.composer { display: grid; grid-template-columns: minmax(0, 1fr) minmax(320px, 0.8fr); gap: 24px; align-items: start; }
	.preview { position: sticky; top: 16px; display: flex; flex-direction: column; gap: 8px; }
	.block { display: flex; flex-direction: column; gap: 10px; padding: 14px 16px; }
	.block-head { align-items: center; }
	.block-kind { font-size: 24px; }
	.block-tools { gap: 6px; }
	.hint code { font-size: 0.85em; }
	.button-row { display: flex; flex-direction: column; gap: 10px; padding: 12px; background: rgb(0 0 0 / 0.25); border-radius: 12px; }
	.button-row .grid-halves { gap: 10px; }
	.add-blocks { gap: 8px; }
	.options { gap: 10px; }
	.message-cell { max-width: 420px; overflow-wrap: anywhere; }
	.actions { align-items: center; }
	.spacer { flex: 1; }
	.callout ul { margin: 4px 0 0; padding-left: 22px; }
	.callout p { margin: 0; }
	.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
	.post-dialog {
		width: min(520px, calc(100vw - 32px));
		padding: 24px 28px;
		color: var(--text);
		background: var(--row-bg);
		border: 0;
		border-radius: var(--radius);
		box-shadow: 0 24px 60px rgb(0 0 0 / 0.6);
	}
	.post-dialog::backdrop { background: rgb(0 0 0 / 0.7); }
	.post-dialog p { margin: 0; }
	@media (max-width: 1100px) {
		.composer { grid-template-columns: 1fr; }
		.preview { position: static; }
	}
</style>
