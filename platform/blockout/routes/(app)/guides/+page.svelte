<script>
	import Note from '#lib/components/Note.svelte';
	import { PROGRAM } from '#lib/data.js';

	const guides = [
		{ title: 'How Crossroads works', text: 'Build, ship, get hours approved, qualify.' },
		{ title: 'Setting up Hackatime', text: 'Install it in your editor and check it is tracking.' },
		{ title: 'What makes a good ship', text: 'Demo links, READMEs and screenshots reviewers love.' },
		{ title: 'Shipping a web app', text: 'Deploy to Vercel, Netlify or GitHub Pages.' },
		{ title: 'Shipping a mobile app', text: 'TestFlight, APKs and Expo links.' },
		{ title: 'Shipping a game', text: 'itch.io builds and web exports.' },
		{ title: 'Shipping hardware', text: 'Video demos, schematics and build photos.' },
		{ title: 'Project ideas', text: 'Starter projects if you are stuck.' }
	];
	const faq = [
		['Who can join?', 'Teenagers aged 13–18 anywhere in the world.'],
		['What counts as a project?', 'Anything technical you build: apps, games, tools, hardware. It must be open source and something people can try.'],
		['How are hours counted?', `Hackatime tracks time in your editor. When you ship, a reviewer approves the hours that went into it. ${PROGRAM.goalHours} approved hours qualifies you for the trip.`],
		['Why were fewer hours approved than I tracked?', 'Reviewers remove time that doesn\'t show up in the project, like idle time or work on something else. Your feedback says why.'],
		['Why was my ship rejected?', 'Most often: no working demo, a README that doesn\'t explain how to run it, or it\'s hard to tell what was built. Fix it and ship again.'],
		['Can I use AI?', 'Yes for help, but declare how. Time where AI wrote the code for you doesn\'t count, and READMEs must be your own words.'],
		['Can I work with friends?', 'TBD.'],
		['Is the trip really free?', 'Yes. Flights, trains, food and places to stay are covered for everyone who qualifies.']
	];
</script>

<div class="stack-lg">
	<div class="page-head"><h1>Guides & FAQ</h1></div>

	<section class="stack">
		<h2>Guides</h2>
		<div class="grid">
			{#each guides as g}
				<a class="card" href="/guides"><strong>{g.title}</strong><span class="small muted">{g.text}</span></a>
			{/each}
		</div>
		<Note from="Beest / Horizons">One shipping guide per project type, each listing common rejection reasons. The Ship button on a project can link to the one for its type.</Note>
	</section>

	<section class="stack">
		<h2>FAQ</h2>
		<div class="card">
			{#each faq as [q, a]}
				<details><summary><strong>{q}</strong></summary><p class="small" style="padding: 8px 0 12px">{a}</p></details>
			{/each}
		</div>
	</section>

	<section class="card" id="parents">
		<h2>For parents</h2>
		<p class="small">Placeholder: safeguarding, supervision on the trip, how travel is booked, and who to contact.</p>
	</section>
</div>
