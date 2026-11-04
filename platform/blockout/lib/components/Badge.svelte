<script>
	import { STATUS } from '#lib/data.js';
	let { status } = $props();
	const s = $derived(STATUS[status] ?? { label: status, tone: 'muted' });
</script>

<span class="badge {s.tone}">{s.label}</span>
