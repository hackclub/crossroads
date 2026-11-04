<script>
	const faq = [
		['How does it work?', 'Build any technical project with Hackatime running. Ship it when it works and people can try it. A reviewer approves the new hours you tracked. Every approved hour pays 30 stamps. The trip ticket costs 1,800 stamps (60 hours), and buying it is how you qualify.'],
		['What counts as a project?', 'Anything technical you build: apps, games, tools, hardware. To ship it needs a description, a public repo, a demo people can try, a screenshot and at least one linked Hackatime project.'],
		['Which hours count?', 'Hackatime time from November 3, 2026 onward, on the Hackatime projects you link. Each Hackatime project can only count toward one of your projects.'],
		['Why were fewer hours approved than I tracked?', 'Reviewers remove time that doesn\'t show up in the project, like idle time. The feedback on your project says why. Time a reviewer cut can\'t be shipped again; time from a rejected ship can.'],
		['Can I ship the same project again?', 'Yes. Each ship only claims the time you tracked since the last one.'],
		['What are boxes and special stamps?', 'You get a box each time your approved hours pass a milestone. Opening it drops a special stamp. Some special stamps do something when you ship, like moving your project to the front of the review queue. The chances are always shown on the Boxes page, and boxes can never be bought.'],
		['Why can\'t I buy anything?', 'Buying needs your identity verified on Hack Club Auth (auth.hackclub.com). Once it is, try again; you don\'t need to sign in again.'],
		['Why can\'t you read my Hackatime?', 'Your Hack Club account needs Slack linked, you need a Hackatime account with the same Slack, and your Hackatime stats need to be public.'],
		['Where do I get help?', 'Ask in #crossroads-help on the Hack Club Slack.']
	];
</script>

<div class="narrow stack">
	<h1 class="page-title">Docs</h1>
	<div class="card">
		{#each faq as [q, a]}
			<details><summary><strong>{q}</strong></summary><p class="small" style="padding: 4px 0 12px">{a}</p></details>
		{/each}
	</div>
</div>
