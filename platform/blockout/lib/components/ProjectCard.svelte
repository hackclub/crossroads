<script>
	import Img from './Img.svelte';
	import Badge from './Badge.svelte';
	let { project, showOwner = false } = $props();
</script>

<a class="card project-card" href="/projects/{project.id}">
	{#if project.hasBanner !== false}
		<Img label="Banner" />
	{:else}
		<div class="img empty"><span>No screenshot yet</span></div>
	{/if}
	<div class="stack-sm">
		<div class="row between">
			<strong>{project.title}</strong>
			<Badge status={project.status} />
		</div>
		{#if showOwner}<span class="muted small">by @{project.owner}</span>{/if}
		<p class="muted small clamp">{project.description || 'No description yet.'}</p>
		<div class="row small muted">
			<span>{project.type}</span>
			<span>·</span>
			{#if project.trackedHours != null}
				<span>{project.trackedHours}h tracked</span><span>·</span>
			{/if}
			<span>{project.approvedHours}h approved</span>
		</div>
	</div>
</a>
