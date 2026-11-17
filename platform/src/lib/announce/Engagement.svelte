<script>
	// How a posted announcement is doing (`engagement` from GET
	// /superadmin/announcements/{id}). Clicks come from the bot in slackbot/,
	// counted by Slack ID, so they include people with no account. Reactions,
	// replies and the channel's size are Slack's, as of loading.
	import { date } from '../format.js';
	import { PAGES } from './slack.js';

	let { engagement: e, publicUrl, onrefresh, refreshing = false } = $props();

	const percent = (n, of) => (of ? Math.round((n / of) * 1000) / 10 + '%' : '–');
	const label = (actionId) => e.buttons.find((b) => b.action_id === actionId)?.text || actionId;
	// "Shop" for a page on the platform, "Station Pass" for a claim link,
	// the address otherwise.
	function goesTo(button) {
		if (button.stamp_name) return `Claim ${button.stamp_name}`;
		if (!button.url) return '';
		if (button.url.startsWith(publicUrl + '/')) {
			const path = button.url.slice(publicUrl.length);
			return PAGES.find(([p]) => p === path)?.[1] ?? path;
		}
		return button.url;
	}
</script>

<div class="stack">
	<div class="row between">
		<h2>How it's doing</h2>
		<button class="btn small ghost" disabled={refreshing} onclick={onrefresh}>{refreshing ? 'Refreshing…' : 'Refresh'}</button>
	</div>

	<div class="tiles">
		<div class="tile"><span class="label">Clicked</span><span class="value">{e.people}</span><span class="small muted">{e.members != null ? `${percent(e.people, e.members)} of ${e.members} in the channel` : 'people'}</span></div>
		<div class="tile"><span class="label">Clicks</span><span class="value">{e.clicks}</span><span class="small muted">on every button</span></div>
		<div class="tile"><span class="label">Reacted</span><span class="value">{e.reactors ?? '–'}</span><span class="small muted">people</span></div>
		<div class="tile"><span class="label">Replies</span><span class="value">{e.replies ?? '–'}</span><span class="small muted">in the thread</span></div>
		<div class="tile"><span class="label">Have accounts</span><span class="value">{e.with_accounts}</span><span class="small muted">of those who clicked</span></div>
		<div class="tile"><span class="label">Signed up</span><span class="value">{e.signed_up_after}</span><span class="small muted">after clicking</span></div>
	</div>

	{#if e.slack_problem}<p class="callout info small">{e.slack_problem}</p>{/if}

	{#if e.reactions?.length}
		<div class="pill-row" aria-label="Reactions">
			{#each e.reactions as r}<span class="badge">:{r.name}: {r.count}</span>{/each}
		</div>
	{/if}

	{#if e.buttons.length}
		<div class="card table-wrap">
			<table>
				<thead><tr><th>Button</th><th>Goes to</th><th>Clicks</th><th>People</th><th>Claimed</th></tr></thead>
				<tbody>
					{#each e.buttons as b}
						<tr>
							<td>{b.text || b.action_id}{#if b.removed} <span class="muted small">(edited out)</span>{/if}</td>
							<td class="goes">{goesTo(b)}</td>
							<td>{b.clicks}</td>
							<td>{b.people}</td>
							<td>{#if b.claims != null}<span title="Claims of this stamp since it was posted, through any link">{b.claims}</span>{:else}<span class="muted">–</span>{/if}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	{#if e.clickers.length}
		<details>
			<summary>Who clicked ({e.clickers.length})</summary>
			<div class="card table-wrap">
				<table>
					<thead><tr><th>Who</th><th>Account</th><th>Clicked</th><th>First click</th></tr></thead>
					<tbody>
						{#each e.clickers as c}
							<tr>
								<td>{c.name ?? c.slack_id}{#if c.name}<span class="muted small"> · {c.slack_id}</span>{/if}</td>
								<td>{#if c.user_id}{c.signed_up_at > c.first_click ? 'Signed up after' : 'Yes'}{:else}<span class="muted">None</span>{/if}</td>
								<td>{c.buttons.map(label).join(', ')}</td>
								<td>{date(c.first_click)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</details>
	{:else}
		<p class="muted small">Nobody's clicked a button yet. Clicks on test messages don't count.</p>
	{/if}
</div>

<style>
	.tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 12px; }
	.tile .small { text-align: center; }
	.goes { max-width: 320px; overflow-wrap: anywhere; }
	h2 { margin: 0; }
</style>
