<script>
	// Everything that's happened on the platform, newest first. Superadmins
	// only (GET /api/superadmin/audit-log; planning/ROUTES.md).
	import { onMount } from 'svelte';
	import { get } from '../api.js';
	import { hours } from '../format.js';

	// Each group's kinds, as the filter shows them. A group's button keeps
	// all of its kinds; a kind's button keeps just that kind.
	const GROUPS = [
		['request', 'Requests', []],
		['sign_in', 'Sign-ins', []],
		['special_stamp', 'Special stamps', [['special_stamp.issued', 'Issued'], ['special_stamp.used', 'Used']]],
		['box', 'Boxes', [['box.granted', 'Granted']]],
		['stamps', 'Stamps', [['stamps.payout', 'Payouts'], ['stamps.bonus', 'Bonuses'], ['stamps.purchase', 'Purchases'], ['stamps.refund', 'Refunds'], ['stamps.adjustment', 'Adjustments']]],
		['role', 'Roles', [['role.changed', 'Changed']]],
		['order', 'Orders', [['order.fulfilled', 'Fulfilled'], ['order.cancelled', 'Cancelled']]]
	];
	const LABELS = Object.fromEntries(GROUPS.flatMap(([key, label, kinds]) => [[key, label], ...kinds.map(([k, l]) => [k, `${label}: ${l.toLowerCase()}`])]));
	const PAGE = 100;

	let kinds = $state([]); // empty: every kind
	let person = $state(null); // { id, name }: what they did or what happened to them
	let events = $state([]);
	let more = $state(false);
	let loading = $state(false);
	let msg = $state(null);

	async function load(append = false) {
		loading = true;
		msg = null;
		const query = new URLSearchParams({ limit: PAGE });
		for (const kind of kinds) query.append('kind', kind);
		if (person) query.set('user', person.id);
		if (append && events.length) query.set('before', events.at(-1).id);
		try {
			const page = await get('/superadmin/audit-log?' + query);
			events = append ? [...events, ...page] : page;
			more = page.length === PAGE;
		} catch (e) {
			msg = e.message;
		} finally {
			loading = false;
		}
	}
	onMount(load);

	// Picking a group drops its kinds, and picking a kind drops its group.
	function toggle(key) {
		const group = key.split('.')[0];
		kinds = kinds.includes(key) ? kinds.filter((k) => k !== key) : [...kinds.filter((k) => k !== group && !k.startsWith(key + '.')), key];
		load();
	}
	function only(id, name) {
		person = id ? { id, name } : null;
		load();
	}

	const when = (iso) => new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', second: '2-digit' });
	const signed = (n) => (n > 0 ? `+${n}` : `${n}`);
	const tone = (status) => (status < 300 ? 'good' : status < 500 ? 'warn' : 'info');

	// One line saying what happened; the raw details are a click away.
	function summary(e) {
		const d = e.details;
		switch (e.kind) {
			case 'request':
				return `${d.method} ${d.route || d.path}${d.unrecorded ? ` (${d.unrecorded} before it weren't recorded: too many too fast)` : ''}`;
			case 'sign_in':
				return `Signed in${d.first_time ? ' for the first time' : ''}${d.dev ? ` (dev, as ${d.role})` : ''} from ${d.ip || 'somewhere'}`;
			case 'special_stamp.issued': {
				const from =
					d.source === 'box'
						? `from box #${d.box_id} (${d.box_type}, ${d.box_milestone ? `the ${d.box_milestone} milestone` : 'an admin gave it'})`
						: d.source === 'claim'
							? 'from a claim link'
							: 'given by an admin';
				return `${d.special_stamp_type} (${d.rarity}) ${from}`;
			}
			case 'special_stamp.used':
				return `${d.special_stamp_type} used on ${d.project} (ship #${d.submission_id})`;
			case 'box.granted':
				return `${d.box_type} (box #${d.box_id}) ${d.milestone ? `for the ${d.milestone} milestone` : 'from an admin'}`;
			case 'role.changed':
				return `${d.from} → ${d.to}`;
		}
		if (e.kind.startsWith('stamps.')) {
			const what = {
				payout: `for ${d.project} (${hours(d.approved_seconds)}, ship #${d.submission_id})`,
				bonus: `bonus on ${d.project} (ship #${d.submission_id})`,
				purchase: `on ${d.item} (order #${d.order_id})`,
				refund: `back for ${d.item} (order #${d.order_id})`,
				adjustment: `“${d.note}”`
			}[e.kind.slice(7)];
			return `${signed(d.amount)} stamps ${what ?? ''}, now ${d.balance}`;
		}
		if (e.kind.startsWith('order.')) return `Order #${d.order_id} (${d.item}): ${d.from} → ${d.to}`;
		return e.kind;
	}
</script>

<div class="stack">
	<div class="filters">
		{#each GROUPS as [key, label, groupKinds]}
			<div class="group" role="group" aria-label={label}>
				<button class="btn small" class:primary={kinds.includes(key)} aria-pressed={kinds.includes(key)} onclick={() => toggle(key)}>{label}</button>
				{#each groupKinds as [kind, kindLabel]}
					<button class="btn small ghost" class:primary={kinds.includes(kind)} aria-pressed={kinds.includes(kind)} onclick={() => toggle(kind)}>{kindLabel}</button>
				{/each}
			</div>
		{/each}
	</div>
	<div class="row small">
		{#if kinds.length || person}
			<span class="muted">
				Showing {kinds.length ? kinds.map((k) => LABELS[k]).join(', ') : 'everything'}{person ? ` by or about ${person.name ?? `user #${person.id}`}` : ''}.
			</span>
			<button class="link-btn small" onclick={() => { kinds = []; only(null); }}>Show everything</button>
		{:else}
			<span class="muted">Showing everything. Pick kinds above, or click a name to see one person.</span>
		{/if}
	</div>
	{#if msg}<p class="callout warn small">{msg}</p>{/if}

	<div class="card table-wrap" class:loading>
		<table>
			<thead><tr><th>When</th><th>What</th><th>By</th><th>About</th><th></th></tr></thead>
			<tbody>
				{#each events as e (e.id)}
					<tr>
						<td class="small nowrap" title={e.at}>{when(e.at)}</td>
						<td>
							<span class="badge info tiny">{LABELS[e.kind] ?? e.kind}</span>
							{#if e.kind === 'request'}<span class="badge {tone(e.details.status)} tiny">{e.details.status}</span>{/if}
							<div class="small">{summary(e)}</div>
						</td>
						<td class="small">
							{#if e.actor_id}<button class="link-btn small" onclick={() => only(e.actor_id, e.actor_name)}>{e.actor_name}</button>{:else}<span class="muted">–</span>{/if}
						</td>
						<td class="small">
							{#if e.user_id}<button class="link-btn small" onclick={() => only(e.user_id, e.user_name)}>{e.user_name}</button>{:else}<span class="muted">–</span>{/if}
						</td>
						<td class="small">
							<details>
								<summary class="muted">#{e.id}</summary>
								<pre>{JSON.stringify(e.details, null, 2)}</pre>
							</details>
						</td>
					</tr>
				{:else}
					<tr><td colspan="5" class="muted small">{loading ? 'Loading…' : 'Nothing yet.'}</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
	{#if more}<div class="row"><button class="btn" disabled={loading} onclick={() => load(true)}>Older</button></div>{/if}
</div>

<style>
	.filters { display: flex; flex-wrap: wrap; gap: 10px 24px; }
	.group { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
	.group .ghost { font-size: 20px; }
	.tiny { font-size: 18px; padding: 0 10px; }
	.nowrap { white-space: nowrap; }
	.loading { opacity: 0.55; transition: opacity 0.2s; }
	pre { max-width: 520px; max-height: 360px; overflow: auto; margin: 6px 0 0; font-size: 16px; white-space: pre-wrap; word-break: break-word; }
</style>
