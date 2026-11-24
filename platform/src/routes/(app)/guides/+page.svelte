<script>
	const hackatime = '<a href="https://hackatime.hackclub.com/" target="_blank" rel="noreferrer">Hackatime</a>';
	const hackatimeSetup = '<a href="https://hackatime.hackclub.com/setup" target="_blank" rel="noreferrer">Hackatime Setup Guide</a>';
	const slackHelp = '<a href="https://hackclub.enterprise.slack.com/archives/C0CD7F4RBK9" target="_blank" rel="noreferrer">#crossroads-help</a>';

	const sections = [
		{
			id: 'overview',
			title: 'What is Crossroads',
			tag: 'START HERE',
			number: '01',
			articles: [
				{
					heading: '1. What Crossroads Is',
					body: `Crossroads is an official <a href="https://hackclub.com/" target="_blank" rel="noreferrer">Hack Club</a> YSWS ("You Ship, We Ship") program running in February 2027 where teenagers travel across Europe by train together, leaving from Berlin.<br/><br/>
					The tagline is <strong>"hack across europe"</strong>: <em>5 days. 6 cities. 30 teenagers.</em><br/><br/>
					You build real technical projects, ship them on the Crossroads platform, and Hack Club gives something back. The reward has two parts: a seat on the train trip across Europe, and a shop where approved hours earn currency to purchase grants, gear, and travel stipends.<br/><br/>
					<strong>The flow in one paragraph:</strong> Build a project and track your active time with ${hackatime}. Ship the project on the Crossroads platform. A reviewer assesses the build and approves your hours. Approved hours convert into currency (30 tickets per hour). Spend tickets on a train pass or shop items.`
				},
				{
					heading: 'Navigating Crossroads Stations',
					body: `The core Crossroads pages are stations along the railroad: <strong>Home &rarr; Shop &rarr; Boxes &rarr; Docs</strong>.<br/>
					You can roll between stations anytime using the station links in the top rail bar, keyboard arrow keys (<strong>&larr;</strong> / <strong>&rarr;</strong>), or horizontal trackpad swiping.`
				},
				{
					heading: 'Guide Conventions & Verifications',
					body: `This guide consolidates official organizer specifications and platform guidelines. If guidelines and an organizer ever disagree, the organizers have final say in ${slackHelp}.<br/>
					The Crossroads core team includes <strong>guac</strong> (lead), <strong>Max</strong> (platform lead), and <strong>raygen</strong> &amp; <strong>1mon</strong> (hardware leads).`
				}
			]
		},
		{
			id: 'quick-facts',
			title: 'Quick Facts & Specs',
			tag: 'OVERVIEW',
			number: '02',
			articles: [
				{
					heading: 'Key Facts at a Glance',
					body: `<p>Everything you need to know about the February 2027 Hack Club journey across Europe:</p>
					<ul>
						<li><strong>Who runs it:</strong> Hack Club</li>
						<li><strong>When is the trip:</strong> February 2027 (exact departure schedule announced soon)</li>
						<li><strong>Starting Point:</strong> Berlin, Germany</li>
						<li><strong>Scale:</strong> 5 days, 6 cities, 30 teenagers</li>
						<li><strong>Ticket Cost:</strong> Starts at 60 approved hours (1,800 tickets)</li>
						<li><strong>Seats Available:</strong> 30 spots as of right now (rolling qualification)</li>
						<li><strong>Eligibility:</strong> Any teenager (18 or younger at ship time) worldwide</li>
						<li><strong>Hour Goal:</strong> Community milestone (500 total community hours)</li>
						<li><strong>Platform:</strong> <a href="https://crossroads.hackclub.com" target="_blank" rel="noreferrer">crossroads.hackclub.com</a></li>
						<li><strong>Source Code:</strong> <a href="https://github.com/hackclub/crossroads" target="_blank" rel="noreferrer">github.com/hackclub/crossroads</a></li>
					</ul>`
				}
			]
		},
		{
			id: 'getting-started',
			title: 'Getting Started Step-by-Step',
			tag: 'WALKTHROUGH',
			number: '03',
			articles: [
				{
					heading: 'Step-by-Step Onboarding',
					body: `<ol>
						<li><strong>Join Hack Club Slack:</strong> Join the community and head to <code>#crossroads</code> and ${slackHelp}.</li>
						<li><strong>Sign in to Crossroads:</strong> Visit <a href="https://crossroads.hackclub.com" target="_blank" rel="noreferrer">crossroads.hackclub.com</a> and sign in with your Hack Club Auth profile.</li>
						<li><strong>Install Hackatime:</strong> Set up your time tracker by visiting ${hackatimeSetup}. Install the extension in your preferred editor (VS Code, Zed, Vim, Neovim, JetBrains, etc.).</li>
						<li><strong>Verify Time Logging:</strong> Open <a href="https://hackatime.hackclub.com/my/projects" target="_blank" rel="noreferrer">hackatime.hackclub.com/my/projects</a> and confirm your active coding appears under your project name, not as "Unknown".</li>
						<li><strong>Build Something Technical:</strong> Create a project that meets the technical criteria.</li>
						<li><strong>Link on Crossroads:</strong> Add your project to your Crossroads dashboard and link it to the matching Hackatime project.</li>
						<li><strong>Ship When Ready:</strong> Once functional, ship the project with repository link, demo URL/video, and clear description.</li>
						<li><strong>Review Queue:</strong> Reviewers verify authenticity, inspect code, and approve qualified hours.</li>
						<li><strong>Spend Tickets:</strong> Approved hours convert into currency (30 tickets/hour). Spend on your train ticket, grants, or equipment.</li>
					</ol>`
				}
			]
		},
		{
			id: 'hours-tracking',
			title: 'Hours & Time Tracking',
			tag: 'TRACKING',
			number: '04',
			articles: [
				{
					heading: 'What Hours Count',
					body: `<ul>
						<li><strong>Approved Hours Only:</strong> Hours must be shipped and approved by reviewers before they can be redeemed in the shop or on train passes. Unshipped or unapproved hours cannot be spent.</li>
						<li><strong>Primary Tracker:</strong> ${hackatime} is the required tracker for software development. For CAD and PCB hardware design, use <strong>Lapse</strong>.</li>
						<li><strong>No Idle Time:</strong> Hackatime automatically pauses when you are idle; only active keystrokes and editor interactions count.</li>
						<li><strong>No Research Time:</strong> Passive reading, browsing documentation, and non-coding planning without active work do not count as tracked hours.</li>
					</ul>`
				},
				{
					heading: 'Start Date & Boundaries',
					body: `Hours tracked prior to the official launch do not count towards Crossroads. Only hours logged after the kickoff/RSVP rollout (November 6 cutoff) count. Organizers cannot retroactively push back start dates for older projects.`
				},
				{
					heading: 'Reshipping, Multiple Projects & Collaboration',
					body: `<ul>
						<li><strong>Reshipping:</strong> You can reship an existing project as you make major updates! When reshipping, describe what you added since the last submission. Only newly logged time since the previous ship is claimed.</li>
						<li><strong>Multiple Projects:</strong> You can combine hours from multiple qualifying projects to reach your total hour goal (e.g., 60+ hours).</li>
						<li><strong>Collaboration:</strong> You can build with partners or siblings. Each collaborator must track their time independently on their own Hackatime account, and maintain frequent Git commits so reviewers can verify individual contributions.</li>
						<li><strong>No Double Dipping:</strong> You cannot ship the same work to Crossroads and another Hack Club YSWS program simultaneously.</li>
					</ul>`
				}
			]
		},
		{
			id: 'what-to-build',
			title: 'What You Can Build',
			tag: 'PROJECTS',
			number: '05',
			articles: [
				{
					heading: 'Permitted Software Projects',
					body: `You can build whatever you can think of, provided it is <strong>technical, substantial, and trackable</strong> with Hackatime. Examples include custom web applications, mobile apps, games (Godot, Unity, Bevy), custom operating systems, compilers, tools, MP3 players, and graphics engines.`
				},
				{
					heading: 'AI Usage Rule (30% Max)',
					body: `Crossroads limits AI-assisted coding to a maximum of <strong>30%</strong> of your total project. Projects generated predominantly or entirely by AI prompts will be rejected. Reviewers check commit histories, complexity jumps, and code understanding. Write your own code, and use AI sparingly as an assistant.`
				},
				{
					heading: 'Hardware & CAD Tracking',
					body: `Hardware projects are fully supported! For CAD modeling, PCB layout in KiCad/Altium, and mechanical builds where Hackatime cannot record editor pulses:
					<ul>
						<li><strong>Track with Lapse:</strong> Use Hack Club's <strong>Lapse</strong> screen-timelapse tool to record your workflow. It generates a verified timelapse video of your work to authenticate design hours.</li>
						<li><strong>Hardware Journals:</strong> Keep an active engineering notebook/journal documenting your design iterations, schematics, component choices, and physical assembly.</li>
					</ul>`
				},
				{
					heading: 'Repository Documentation (README)',
					body: `Every shipped project requires a clean, informative <code>README.md</code> in the repository explaining what the project is, how it works, how to run/build it locally, and your personal design journey. READMEs must be written by you, not generated by AI.`
				}
			]
		},
		{
			id: 'reviews',
			title: 'Review Process & Safety',
			tag: 'REVIEWS',
			number: '06',
			articles: [
				{
					heading: 'Review Stages',
					body: `When you ship, your project enters the review pipeline, which features three structured checkpoints:<br/>
					<ol>
						<li><strong>Technical Review:</strong> Reviewers verify the repository, run the code or demo, inspect commit history, and examine Hackatime heartbeats.</li>
						<li><strong>Anti-Fraud & Authenticity Verification:</strong> Automated and manual checks examine AI usage ratios, duplicate submissions, and time-tracking anomalies.</li>
						<li><strong>HQ Sign-Off:</strong> Final validation and ticket issuance to the participant's account.</li>
					</ol>`
				},
				{
					heading: 'Review Queue & Decisions',
					body: `Review speed depends on volunteer queue volume. Reviewers evaluate projects thoroughly to maintain fairness. Special stamps (such as Priority Review) can speed up review order.<br/><br/>
					If a project is rejected or requires changes, feedback will be provided. You can address the notes, improve your build, and resubmit, or reach out in ${slackHelp} for clarification.`
				}
			]
		},
		{
			id: 'shop-currency',
			title: 'Tickets, Shop & Grants',
			tag: 'CURRENCY',
			number: '07',
			articles: [
				{
					heading: 'Currency Conversion',
					body: `Approved hours convert to currency at a rate of <strong>30 tickets per 1 approved hour</strong>. A standard 60-hour ticket equals <strong>1,800 tickets</strong>.`
				},
				{
					heading: 'Ticket Batches & Price Locking',
					body: `<ul>
						<li><strong>Batch Releases:</strong> Train tickets are released in batches of 6 seats.</li>
						<li><strong>Batch Price Increments:</strong> Each time a batch sells out, the subsequent batch ticket price increases by 5 hours (150 tickets).</li>
						<li><strong>Price Lock:</strong> You can spend 5 hours worth of tickets (150 tickets) to lock your ticket price at the current batch tier while you finish building your project!</li>
						<li><strong>Refunds:</strong> Tickets can be refunded back to currency if you determine you cannot attend.</li>
					</ul>`
				},
				{
					heading: 'Shop Items & Grants',
					body: `The Crossroads shop features equipment, hardware components, travel stipends, and specialized project grants (e.g., PC hardware, 3D printing, electronics):
					<ul>
						<li><strong>Grants:</strong> Provide reimbursement funds for approved categories. All purchases made through grants must have itemized receipts submitted via Hack Club Bank (HCB).</li>
						<li><strong>Fulfillment & Shipping:</strong> Physical items and equipment are shipped to the verified address on your profile. Where international shipping is complex, local fulfillment or cash grants are prioritized.</li>
					</ul>`
				}
			]
		},
		{
			id: 'europe-trip',
			title: 'The Europe Journey',
			tag: 'THE TRIP',
			number: '08',
			articles: [
				{
					heading: 'The Train Trip Across Europe',
					body: `In February 2027, 30 qualified teenagers will board trains together in Berlin, Germany, embarking on a 5-day voyage across 6 European cities to collaborate, build, and explore.`
				},
				{
					heading: 'What is Covered',
					body: `For teenagers who claim a trip pass:<br/>
					<ul>
						<li><strong>All European Train Passes:</strong> Complete rail transport across all 6 destination cities.</li>
						<li><strong>Lodging & Accommodations:</strong> Safe, vetted hostel/hotel arrangements with Hack Club chaperones.</li>
						<li><strong>Meals & Food:</strong> Full meal coverage throughout the journey.</li>
					</ul>`
				},
				{
					heading: 'Travel Stipends & Flights',
					body: `International flights to Berlin are not directly booked by the rail ticket. Instead, participants can redeem currency in the shop for <strong>Travel & Flight Stipends</strong> to fund their transport to the departure station.`
				},
				{
					heading: 'Visas & Travel Documents',
					body: `<ul>
						<li><strong>Schengen Area:</strong> Berlin and the travel destinations are within the Schengen Zone. One Schengen visa covers travel between member states.</li>
						<li><strong>EU / Schengen Citizens:</strong> European participants generally only need their national ID or passport.</li>
						<li><strong>Visa Support:</strong> Hack Club will provide official invitation letters and visa assistance to participants once hour qualifications are verified. Do not apply for visas before receiving your official paperwork.</li>
					</ul>`
				}
			]
		},
		{
			id: 'rules-summary',
			title: 'Rules at a Glance',
			tag: 'RULES',
			number: '09',
			articles: [
				{
					heading: 'Core Program Rules',
					body: `<ul>
						<li><strong>Age Limit:</strong> 18 or younger at the moment of shipping.</li>
						<li><strong>No Double Dipping:</strong> You cannot submit the same code to multiple Hack Club YSWS programs.</li>
						<li><strong>AI Coding:</strong> Maximum 30% of project may use AI assistance. Pure AI projects are rejected.</li>
						<li><strong>Start Date:</strong> Hours prior to the kickoff/RSVP rollout do not count.</li>
						<li><strong>Hour Spending:</strong> Only shipped and approved hours convert into spendable tickets.</li>
						<li><strong>Reshipping:</strong> Permitted; you must document what features are new since last ship.</li>
						<li><strong>Collaboration:</strong> Allowed; each teammate tracks time independently and commits regularly.</li>
						<li><strong>Multiple Projects:</strong> Permitted; hours across qualifying projects aggregate together.</li>
						<li><strong>Active Time Only:</strong> Hackatime idle time and non-coding research do not count.</li>
						<li><strong>Grant Receipts:</strong> Itemized receipts are required for all money spent from grants.</li>
						<li><strong>Code of Conduct:</strong> Hack Club Community Guidelines are strictly enforced.</li>
					</ul>`
				}
			]
		},
		{
			id: 'troubleshooting',
			title: 'Troubleshooting & Support',
			tag: 'SUPPORT',
			number: '10',
			articles: [
				{
					heading: 'Common Issues & Solutions',
					body: `<ul>
						<li><strong>Hackatime shows more hours than Crossroads:</strong> Crossroads only tallies hours logged after the kickoff date. Check project start timestamps.</li>
						<li><strong>Project shows as "Unknown" in Hackatime:</strong> Your editor plugin is not transmitting the active project workspace folder name. Open the root folder in VS Code / Zed and verify plugin status. Visit ${hackatimeSetup} for setup steps.</li>
						<li><strong>Hours marked "not shipped":</strong> Ensure your project is linked correctly and submitted through the ship modal on your dashboard.</li>
						<li><strong>Review takes time:</strong> Submissions undergo multi-stage technical and anti-fraud review. Review times vary with queue size.</li>
						<li><strong>Hardware hours not registering:</strong> Record physical/CAD workflow using <strong>Lapse</strong> and attach the timelapse session to your project.</li>
						<li><strong>Working on multiple computers:</strong> Configure the identical Hackatime API key across all machines and use the same Git repository.</li>
					</ul>`
				},
				{
					heading: 'Asking for Help in #crossroads-help',
					body: `When posting for assistance in ${slackHelp}, include:<br/>
					<ol>
						<li>What you expected to happen vs what happened.</li>
						<li>Your Crossroads project link and Hackatime project name.</li>
						<li>A screenshot if encountering UI or tracking errors.</li>
						<li>Whether you are reporting a bug or requesting clarification on a rule.</li>
					</ol>`
				}
			]
		},
		{
			id: 'faq',
			title: 'Frequently Asked Questions',
			tag: 'FAQ',
			number: '11',
			faqItems: [
				{
					q: 'How many tickets do I earn per hour?',
					a: 'Every approved hour awards exactly 30 tickets. The rail pass for the Europe trip costs 1,800 tickets (60 approved hours).'
				},
				{
					q: 'Can I ship multiple projects?',
					a: 'Yes. You can work on and ship several distinct projects. Your total approved hours and ticket earnings aggregate across your entire profile.'
				},
				{
					q: 'Can I update and re-ship an existing project?',
					a: 'Yes. Re-shipping an approved project claims newly tracked Hackatime hours accumulated since the last approval.'
				},
				{
					q: 'What are boxes and special stamps?',
					a: 'Boxes are earned upon reaching approved hour milestones (e.g., 10, 30, and 50 hours). Opening a box drops a special stamp. Certain stamps provide special effects, such as moving a submission to priority review queue.'
				},
				{
					q: 'Can special stamps or boxes be purchased?',
					a: 'No. Boxes and special stamps can only be unlocked by reaching milestone achievements. They cannot be purchased.'
				},
				{
					q: 'Why are my Hackatime hours not appearing on the platform?',
					a: `Confirm that your Slack account is linked on Hack Club Auth, that your Hackatime profile shares that Slack ID, and that public stats are turned on in your Hackatime settings. Follow the ${hackatimeSetup} for full instructions.`
				},
				{
					q: 'Where can I reach out for assistance?',
					a: `Ask questions and get technical support in ${slackHelp} on the official Hack Club Slack.`
				}
			]
		}
	];

	let activeSection = $state('overview');
	const current = $derived(sections.find((s) => s.id === activeSection) || sections[0]);

	function onKeyDown(e) {
		if (e.target instanceof HTMLElement && (e.target.isContentEditable || /^(INPUT|TEXTAREA|SELECT|BUTTON)$/.test(e.target.tagName))) return;
		if (document.querySelector('dialog[open]')) return;

		if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
			e.preventDefault();
			const idx = sections.findIndex((s) => s.id === activeSection);
			if (idx === -1) return;
			const delta = e.key === 'ArrowDown' ? 1 : -1;
			const nextIdx = (idx + delta + sections.length) % sections.length;
			activeSection = sections[nextIdx].id;
		}
	}
</script>

<svelte:window onkeydown={onKeyDown} />

<div class="docs-layout">
	<!-- Left Animated Rail Navigation -->
	<aside class="docs-nav-sidebar">
		<div class="nav-brand">
			<div class="rail-indicator-dot"></div>
			<div class="nav-brand-text">
				<span class="brand-title">CROSSROADS</span>
				<span class="brand-subtitle">SYSTEM DOCUMENTATION</span>
			</div>
		</div>

		<nav class="nav-links" aria-label="Documentation sections">
			{#each sections as s}
				<button
					type="button"
					class="nav-tab"
					class:active={activeSection === s.id}
					onclick={() => (activeSection = s.id)}
				>
					<span class="rail-line" aria-hidden="true"></span>
					<span class="tab-number">{s.number}</span>
					<div class="tab-meta">
						<span class="tab-tag">{s.tag}</span>
						<span class="tab-label">{s.title}</span>
					</div>
				</button>
			{/each}
		</nav>

		<div class="nav-footer">
			<span class="footer-note">Need assistance?</span>
			<a
				href="https://hackclub.enterprise.slack.com/archives/C0CD7F4RBK9"
				target="_blank"
				rel="noreferrer"
				class="footer-link"
			>
				#crossroads-help
			</a>
		</div>
	</aside>

	<!-- Main Content Panel -->
	<main class="docs-main-panel">
		<header class="panel-header">
			<div class="header-meta">
				<span class="section-tag">{current.tag}</span>
				<span class="section-index">SECTION {current.number} / {String(sections.length).padStart(2, '0')}</span>
			</div>
			<h1 class="panel-heading">{current.title}</h1>
		</header>

		<div class="panel-content">
			{#if current.articles}
				<div class="articles-stack">
					{#each current.articles as art}
						<section class="doc-block">
							<h2 class="doc-subheading">{art.heading}</h2>
							<div class="doc-body">{@html art.body}</div>
						</section>
					{/each}
				</div>
			{/if}

			{#if current.faqItems}
				<div class="faq-stack">
					{#each current.faqItems as item}
						<details class="faq-card">
							<summary class="faq-header">
								<span class="faq-title">{item.q}</span>
								<span class="faq-icon" aria-hidden="true">+</span>
							</summary>
							<div class="faq-content">
								{@html item.a}
							</div>
						</details>
					{/each}
				</div>
			{/if}
		</div>
	</main>
</div>

<style>
	.docs-layout {
		display: grid;
		grid-template-columns: 290px 1fr;
		gap: 32px;
		max-width: var(--content);
		margin: 0 auto;
		padding: 24px 16px 80px;
		align-items: start;
	}

	/* Navigation Sidebar */
	.docs-nav-sidebar {
		background: var(--row-bg);
		border: 1px solid var(--deep);
		border-radius: 16px;
		padding: 20px 16px;
		position: sticky;
		top: 24px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		box-shadow: 0 4px 20px rgb(0 0 0 / 0.45);
	}

	.nav-brand {
		display: flex;
		align-items: center;
		gap: 12px;
		padding-bottom: 16px;
		border-bottom: 1px solid var(--deep);
	}

	.rail-indicator-dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: var(--warn);
		box-shadow: 0 0 10px var(--warn);
		flex-shrink: 0;
	}

	.nav-brand-text {
		display: flex;
		flex-direction: column;
	}

	.brand-title {
		font-size: 16px;
		font-weight: 600;
		letter-spacing: 1.5px;
		color: var(--text);
		line-height: 1.1;
	}

	.brand-subtitle {
		font-size: 11px;
		letter-spacing: 0.8px;
		color: var(--faint);
		margin-top: 2px;
	}

	.nav-links {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.nav-tab {
		display: flex;
		align-items: center;
		gap: 12px;
		width: 100%;
		padding: 10px 12px;
		border-radius: 10px;
		background: transparent;
		border: 1px solid transparent;
		color: var(--muted);
		text-align: left;
		cursor: pointer;
		position: relative;
		transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
		font-family: inherit;
	}

	.nav-tab:hover {
		background: rgb(255 255 255 / 0.05);
		color: var(--text);
		transform: translateX(4px);
	}

	.nav-tab.active {
		background: var(--navy);
		border-color: rgb(106 166 245 / 0.35);
		color: var(--text);
	}

	.rail-line {
		position: absolute;
		left: 0;
		top: 15%;
		height: 70%;
		width: 3px;
		border-radius: 2px;
		background: var(--warn);
		opacity: 0;
		transform: scaleY(0);
		transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
	}

	.nav-tab.active .rail-line {
		opacity: 1;
		transform: scaleY(1);
	}

	.tab-number {
		font-family: ui-monospace, SFMono-Regular, monospace;
		font-size: 12px;
		color: var(--faint);
		width: 20px;
		flex-shrink: 0;
	}

	.nav-tab.active .tab-number {
		color: var(--info);
	}

	.tab-meta {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.tab-tag {
		font-size: 10px;
		letter-spacing: 0.6px;
		color: var(--faint);
		text-transform: uppercase;
	}

	.nav-tab.active .tab-tag {
		color: var(--warn);
	}

	.tab-label {
		font-size: 15px;
		font-weight: 500;
		line-height: 1.25;
		color: var(--text);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.nav-footer {
		padding-top: 14px;
		border-top: 1px solid var(--deep);
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.footer-note {
		font-size: 12px;
		color: var(--faint);
	}

	.footer-link {
		font-family: ui-monospace, SFMono-Regular, monospace;
		font-size: 13px;
		color: var(--info);
		text-decoration: none;
		transition: color 0.15s ease;
	}

	.footer-link:hover {
		color: var(--warn);
		text-decoration: underline;
	}

	/* Main Content Panel */
	.docs-main-panel {
		display: flex;
		flex-direction: column;
		gap: 24px;
		min-width: 0;
	}

	.panel-header {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding-bottom: 18px;
		border-bottom: 1px solid var(--deep);
	}

	.header-meta {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.section-tag {
		font-family: ui-monospace, SFMono-Regular, monospace;
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		padding: 3px 8px;
		border-radius: 6px;
		background: var(--navy);
		color: var(--info);
		border: 1px solid rgb(106 166 245 / 0.3);
	}

	.section-index {
		font-family: ui-monospace, SFMono-Regular, monospace;
		font-size: 12px;
		color: var(--faint);
	}

	.panel-heading {
		font-family: 'Crimson Pro', Georgia, serif;
		font-size: clamp(34px, 3.5vw, 44px);
		font-weight: 600;
		line-height: 1.15;
		color: var(--text);
		letter-spacing: -0.5px;
	}

	.panel-content {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.articles-stack {
		display: flex;
		flex-direction: column;
		gap: 18px;
	}

	.doc-block {
		background: var(--row-bg);
		border: 1px solid var(--deep);
		border-radius: 14px;
		padding: 24px 28px;
		box-shadow: 0 4px 16px rgb(0 0 0 / 0.3);
		transition: border-color 0.2s ease;
	}

	.doc-block:hover {
		border-color: rgb(255 255 255 / 0.22);
	}

	.doc-subheading {
		font-family: 'Crimson Pro', Georgia, serif;
		font-size: 26px;
		font-weight: 600;
		color: var(--text);
		margin-bottom: 12px;
		line-height: 1.2;
	}

	.doc-body {
		font-size: 17px;
		line-height: 1.6;
		color: var(--muted);
	}

	.doc-body :global(p) {
		margin-bottom: 12px;
	}

	.doc-body :global(p:last-child) {
		margin-bottom: 0;
	}

	.doc-body :global(ul),
	.doc-body :global(ol) {
		margin: 10px 0;
		padding-left: 24px;
	}

	.doc-body :global(li) {
		margin-bottom: 6px;
	}

	.doc-body :global(code) {
		background: rgb(255 255 255 / 0.08);
		padding: 2px 6px;
		border-radius: 4px;
		font-family: ui-monospace, SFMono-Regular, monospace;
		font-size: 14px;
		color: var(--warn);
	}

	.doc-body :global(a) {
		color: var(--info);
		text-decoration: underline;
		text-underline-offset: 3px;
	}

	.doc-body :global(strong) {
		color: var(--text);
		font-weight: 600;
	}

	/* FAQ Cards */
	.faq-stack {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.faq-card {
		background: var(--row-bg);
		border: 1px solid var(--deep);
		border-radius: 12px;
		overflow: hidden;
		transition: border-color 0.2s ease, background 0.2s ease;
	}

	.faq-card[open] {
		border-color: rgb(106 166 245 / 0.4);
		background: linear-gradient(rgb(55 55 55 / 0.95), rgb(55 55 55 / 0.95)), var(--leak);
	}

	.faq-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 16px 20px;
		cursor: pointer;
		font-family: 'Crimson Pro', Georgia, serif;
		font-size: 21px;
		font-weight: 500;
		color: var(--text);
		user-select: none;
		list-style: none;
	}

	.faq-header::-webkit-details-marker {
		display: none;
	}

	.faq-icon {
		font-size: 20px;
		color: var(--faint);
		transition: transform 0.2s ease, color 0.2s ease;
		line-height: 1;
		flex-shrink: 0;
	}

	.faq-card[open] .faq-icon {
		transform: rotate(45deg);
		color: var(--warn);
	}

	.faq-content {
		padding: 0 20px 18px;
		font-size: 17px;
		line-height: 1.55;
		color: var(--muted);
		border-top: 1px solid rgb(255 255 255 / 0.06);
		padding-top: 12px;
	}

	.faq-content :global(a) {
		color: var(--info);
		text-decoration: underline;
		text-underline-offset: 3px;
	}

	@media (max-width: 820px) {
		.docs-layout {
			grid-template-columns: 1fr;
			gap: 20px;
		}

		.docs-nav-sidebar {
			position: static;
		}

		.nav-links {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
			gap: 8px;
		}

		.nav-tab {
			padding: 8px 10px;
		}
	}
</style>
