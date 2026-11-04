<script>
	import Note from '#lib/components/Note.svelte';
	import { PROGRAM } from '#lib/data.js';
</script>

<div class="centered-screen">
	<div class="card stack">
		<h1>Crossroads</h1>
		<p class="muted">
			Build for {PROGRAM.goalHours} hours, then travel across Europe by train with {PROGRAM.spots}
			other teens, {PROGRAM.tripDates}.
		</p>
		<a class="btn primary" href="/onboarding">Sign in with Hack Club</a>
		<p class="small muted">
			New here? Signing in creates your account. <a href="/guides">What is Crossroads?</a>
		</p>
		<hr />
		<a class="small" href="/home">Skip to the dashboard (returning participant)</a>
		<Note from="Fallen, Beest, Horizons">
			Every program signs in with Hack Club Auth, which also holds the participant's address,
			birthday and ID check, so the platform never asks for them itself. The marketing page lives in
			<code>main page/</code>; this is where its "Get Started" buttons should land.
		</Note>
	</div>
</div>
