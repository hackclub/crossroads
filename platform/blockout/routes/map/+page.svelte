<script>
	// Blockout-only: every page in one place. Delete with the blockout bar.
	const sections = [
		['Start', [['/', 'Sign in'], ['/onboarding', 'Onboarding']]],
		['Dashboard', [['/home', 'Home'], ['/notifications', 'Notifications'], ['/settings', 'Settings']]],
		['Projects', [
			['/projects', 'My projects'],
			['/projects/new', 'New project'],
			['/projects/metro-map', 'Project (shipped)'],
			['/projects/pixel-garden', 'Project (changes needed / in review)'],
			['/projects/study-bot', 'Project (not shipped, blocked)'],
			['/projects/metro-map/edit', 'Edit project'],
			['/projects/metro-map/ship', 'Ship flow'],
			['/projects/rail-planner', "Someone else's project"]
		]],
		['Community', [['/explore', 'Explore'], ['/leaderboard', 'Leaderboard'], ['/u/jordan', 'My profile'], ['/u/ada', "Someone else's profile"]]],
		['Shop', [['/shop', 'Shop'], ['/shop/keyboard', 'Shop item'], ['/shop/rail-pass', 'Shop item (trip only)'], ['/shop/orders', 'My orders']]],
		['Trip and help', [['/trip', 'The trip'], ['/guides', 'Guides & FAQ']]]
	];
</script>

<div class="main narrow">
	<div class="page-head">
		<div class="stack-sm">
			<h1>Page map</h1>
			<p class="muted">Every page in the blockout. Switch the scenario in the bar above to see each one at a different stage.</p>
		</div>
	</div>
	<div class="stack">
		{#each sections as [title, links]}
			<section class="card">
				<h2>{title}</h2>
				<ul class="list">
					{#each links as [href, label]}
						<li><a href={href} style="flex: 1">{label}</a><span class="muted small">{href}</span></li>
					{/each}
				</ul>
			</section>
		{/each}
	</div>
</div>
