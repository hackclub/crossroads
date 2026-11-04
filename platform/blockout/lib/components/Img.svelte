<script>
	// Image placeholder: a crossed box with a label.
	let { label = 'Image', ratio = '16 / 9', round = false } = $props();
</script>

<div class="img" class:round style="aspect-ratio: {ratio}">
	<span>{label}</span>
</div>
