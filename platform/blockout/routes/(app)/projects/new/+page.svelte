<script>
	import ProjectForm from '#lib/components/ProjectForm.svelte';
	import Note from '#lib/components/Note.svelte';
</script>

<div class="main narrow" style="padding: 0">
	<div class="crumbs"><a href="/projects">My projects</a> / New</div>
	<div class="page-head"><h1>New project</h1></div>
	<div class="stack">
		<Note from="Fallen">Only the name is needed to create a project. Everything else can be added later and is checked when you ship.</Note>
		<ProjectForm />
	</div>
</div>
