<script>
	import { view } from '#lib/state.svelte.js';
	import { getData } from '#lib/data.js';
	const d = $derived(getData(view.scenario));
</script>

<div class="page-head"><h1>Notifications</h1>{#if d.notifications.length}<button class="btn ghost small">Mark all read</button>{/if}</div>

{#if d.notifications.length}
	<div class="card">
		<ul class="list">
			{#each d.notifications as n}
				<li><span class="muted small" style="width: 56px">{n.date}</span><a href={n.href} style="flex: 1">{n.text}</a></li>
			{/each}
		</ul>
	</div>
{:else}
	<div class="card dashed"><p class="muted">Nothing yet. Review results and order updates show up here and in Slack.</p></div>
{/if}
