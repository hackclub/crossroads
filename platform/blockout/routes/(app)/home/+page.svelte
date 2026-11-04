<script>
	import HourBar from '#lib/components/HourBar.svelte';
	import ProjectCard from '#lib/components/ProjectCard.svelte';
	import Note from '#lib/components/Note.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, shipBlockers, PROGRAM } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	const days = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
	const maxMin = $derived(Math.max(...d.week, 60));

	// The single most useful thing to do next, in priority order.
	const nextStep = $derived.by(() => {
		if (!d.me.hackatime)
			return { title: 'Connect Hackatime', text: 'Your hours can only be approved if Hackatime tracks them.', href: '/settings', cta: 'Connect' };
		if (!d.projects.length)
			return { title: 'Start your first project', text: 'Pick anything technical you want to build.', href: '/projects/new', cta: 'New project' };
		if (d.qualified)
			return { title: "You've qualified for the trip!", text: 'Confirm your spot and fill in your travel details.', href: '/trip', cta: 'Confirm my spot' };
		const fix = d.projects.find((p) => p.status === 'changes_needed');
		if (fix)
			return { title: `${fix.title} needs changes`, text: 'A reviewer left feedback. Fix it and ship again.', href: `/projects/${fix.id}`, cta: 'See feedback' };
		const ready = d.projects.find((p) => !shipBlockers(p, d.me).length);
		if (ready)
			return { title: `${ready.title} is ready to ship`, text: 'Ship it to get its new hours approved.', href: `/projects/${ready.id}/ship`, cta: 'Ship it' };
		return { title: 'Keep building', text: `Track ${PROGRAM.minShipHours}h on a project, then ship it.`, href: '/projects', cta: 'My projects' };
	});

	const setup = $derived([
		{ done: true, label: 'Sign in with Hack Club' },
		{ done: d.me.slack, label: 'Join Slack' },
		{ done: d.me.hackatime, label: 'Connect Hackatime', href: '/settings' },
		{ done: d.me.profileComplete, label: 'Verify your ID and add your address', href: '/settings' },
		{ done: d.projects.length > 0, label: 'Create a project', href: '/projects/new' },
		{ done: d.projects.some((p) => p.ships.length), label: 'Ship your first project' }
	]);
	const setupLeft = $derived(setup.filter((s) => !s.done).length);
</script>

<div class="stack-lg">
	<div class="page-head">
		<div class="stack-sm">
			<h1>Welcome back, {d.me.name}</h1>
			<p class="muted">{PROGRAM.daysToDeadline} days left to qualify · Ship deadline {PROGRAM.shipDeadline}</p>
		</div>
		<a class="btn primary" href="/projects/new">New project</a>
	</div>

	<section class="card">
		<div class="card-title">
			<h2>Your trip progress</h2>
			<a class="small" href="/trip">About the trip</a>
		</div>
		<HourBar hours={d.hours} />
		<Note from="Beest / Shipwrecked / Horizons">
			One stacked bar: approved hours count; in-review and unshipped hours show what's coming.
			Spending {PROGRAM.currency} in the shop never lowers this.
		</Note>
	</section>

	<section class="callout {d.qualified ? 'good' : 'info'}">
		<div class="row between">
			<div class="stack-sm">
				<strong>Next step: {nextStep.title}</strong>
				<span class="small">{nextStep.text}</span>
			</div>
			<a class="btn" href={nextStep.href}>{nextStep.cta}</a>
		</div>
	</section>

	<div class="grid-2">
		<div class="stack">
			{#if setupLeft > 0}
				<section class="card">
					<div class="card-title">
						<h2>Get set up</h2>
						<span class="muted small">{setup.length - setupLeft} of {setup.length} done</span>
					</div>
					<ul class="checklist">
						{#each setup as item}
							<li>
								<span class={item.done ? 'ok' : 'no'}>{item.done ? '✓' : '○'}</span>
								{#if item.href && !item.done}<a href={item.href}>{item.label}</a>{:else}<span class:muted={item.done}>{item.label}</span>{/if}
							</li>
						{/each}
					</ul>
					<Note from="Flavortown">Setup checklist only shows until everything is done.</Note>
				</section>
			{/if}

			<section class="stack">
				<div class="card-title">
					<h2>My projects</h2>
					<a class="small" href="/projects">See all</a>
				</div>
				{#if d.projects.length}
					<div class="grid">
						{#each d.projects.slice(0, 3) as project}
							<ProjectCard {project} />
						{/each}
						<a class="card dashed" href="/projects/new">
							<strong>+ New project</strong>
							<span class="muted small">Start something new</span>
						</a>
					</div>
				{:else}
					<div class="card dashed">
						<strong>No projects yet</strong>
						<p class="muted small">Start one, link it to Hackatime, and your hours begin counting.</p>
						<div class="row"><a class="btn primary" href="/projects/new">Create a project</a><a class="btn ghost" href="/guides">Need ideas?</a></div>
					</div>
				{/if}
			</section>

			<section class="card">
				<div class="card-title">
					<h2>Happening now</h2>
					<a class="small" href="/explore">Explore</a>
				</div>
				<ul class="list small">
					<li><span class="avatar">A</span><span><strong>@ada</strong> shipped Rail Planner <span class="muted">· 2h ago</span></span></li>
					<li><span class="avatar">K</span><span><strong>@kofi</strong> posted a devlog on Pocket Synth <span class="muted">· 5h ago</span></span></li>
					<li><span class="avatar">L</span><span><strong>@lin</strong> qualified for the trip <span class="muted">· yesterday</span></span></li>
				</ul>
				<Note from="Horizons">Social proof ("N people coding today") keeps the dashboard feeling alive.</Note>
			</section>
		</div>

		<aside class="stack">
			<section class="card">
				<div class="card-title">
					<h2>This week</h2>
					<span class="muted small">Hackatime</span>
				</div>
				<div class="week">
					{#each d.week as min, i}
						<div>
							<span class="col" style="height: {(min / maxMin) * 70}%"></span>
							{days[i]}
						</div>
					{/each}
				</div>
				<div class="row between small">
					<span><strong>{d.me.streak}-day streak</strong></span>
					<span class="muted">{d.me.hackatimeTodayMin} min today</span>
				</div>
				{#if d.me.streak > 0}
					<p class="small muted">Code 15 more minutes today to keep your streak.</p>
				{/if}
			</section>

			<section class="card">
				<div class="card-title">
					<h2>Balance</h2>
					<a class="small" href="/shop">Shop</a>
				</div>
				<p class="big">{d.balance} <span class="small muted">{PROGRAM.currency}</span></p>
				<p class="small muted">{PROGRAM.tokensPerHour} {PROGRAM.currency} per approved hour.</p>
			</section>

			<section class="card">
				<h2>Announcements</h2>
				<ul class="list small">
					{#each d.announcements as a}
						<li class="stack-sm"><strong>{a.title}</strong><span class="muted">{a.date} · {a.text}</span></li>
					{/each}
				</ul>
			</section>

			<section class="card">
				<h2>Need help?</h2>
				<p class="small">Ask in <strong>#crossroads-help</strong> on Slack or read the guides.</p>
				<div class="row"><a class="btn small" href="/guides">Guides & FAQ</a></div>
			</section>
		</aside>
	</div>
</div>
