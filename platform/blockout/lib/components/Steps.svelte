<script>
	// Numbered steps for wizards (onboarding, ship).
	let { steps, current } = $props();
</script>

<ol class="steps">
	{#each steps as step, i}
		<li class:done={i < current} class:current={i === current}>
			<span class="num">{i < current ? '✓' : i + 1}</span>{step}
		</li>
	{/each}
</ol>
