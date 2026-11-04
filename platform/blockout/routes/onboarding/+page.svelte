<script>
	import Steps from '#lib/components/Steps.svelte';
	import Note from '#lib/components/Note.svelte';
	import { PROGRAM, PROJECT_TYPES } from '#lib/data.js';

	const steps = ['Welcome', 'Join Slack', 'Hackatime', 'About you', 'First project'];
	let step = $state(0);
	let heartbeat = $state(false);
	let knowsProject = $state(null);

	const next = () => (step = Math.min(step + 1, steps.length));
	const back = () => (step = Math.max(step - 1, 0));
</script>

<div class="centered-screen">
	<div class="card stack">
		{#if step < steps.length}
			<Steps {steps} current={step} />
		{/if}

		{#if step === 0}
			<h1>Welcome to Crossroads</h1>
			<p>Here's how it works:</p>
			<ol class="stack-sm">
				<li><strong>Build</strong> any technical project, with Hackatime tracking your time.</li>
				<li><strong>Ship</strong> it: submit it for review once it works and people can try it.</li>
				<li><strong>Get hours approved.</strong> Every approved hour counts toward the trip and earns {PROGRAM.currency} for the shop.</li>
				<li><strong>Qualify</strong> at {PROGRAM.goalHours} approved hours and join the train across Europe, {PROGRAM.tripDates}.</li>
			</ol>
			<p class="muted small">Can't travel? You still earn {PROGRAM.currency} for every approved hour.</p>
			<div class="row"><button class="btn primary" onclick={next}>Let's go</button></div>
			<Note from="Beest / Flavortown">One-screen summary of the loop before any setup.</Note>
		{:else if step === 1}
			<h2>Join the Hack Club Slack</h2>
			<p>This is where you get help, share progress and hear about events. Post in #crossroads-help any time.</p>
			<div class="row">
				<a class="btn" href="https://hackclub.com/slack" target="_blank" rel="noreferrer">Open Slack</a>
				<button class="btn primary" onclick={next}>I've joined</button>
			</div>
			<button class="link-btn small muted" onclick={next}>Already a member? Link my account</button>
		{:else if step === 2}
			<h2>Set up Hackatime</h2>
			<p>Hackatime tracks how long you spend coding. Only tracked hours can be approved, so this step is required.</p>
			<ol class="stack-sm small">
				<li>Sign in at <a href="https://hackatime.hackclub.com/" target="_blank" rel="noreferrer">hackatime.hackclub.com</a>.</li>
				<li>Install the extension for your editor.</li>
				<li>Write a few lines of code, then come back.</li>
			</ol>
			<div class="callout {heartbeat ? 'good' : 'info'}">
				{heartbeat ? '✓ We got your first heartbeat. You are all set.' : 'Waiting for your first heartbeat…'}
			</div>
			<div class="row">
				<button class="btn ghost" onclick={back}>Back</button>
				{#if heartbeat}
					<button class="btn primary" onclick={next}>Continue</button>
				{:else}
					<button class="btn" onclick={() => (heartbeat = true)}>Check again</button>
				{/if}
			</div>
			<Note from="Shipwrecked">Confirms a heartbeat arrived instead of trusting a checkbox.</Note>
		{:else if step === 3}
			<h2>About you</h2>
			<p class="muted small">Your name, birthday and address come from your Hack Club account.</p>
			<label class="field"><span>Display name</span><input type="text" value="Jordan" /></label>
			<label class="field"><span>Country</span><input type="text" value="Germany" /></label>
			<fieldset>
				<legend>What are you here for?</legend>
				<label class="check"><input type="radio" name="intent" checked /> The trip</label>
				<label class="check"><input type="radio" name="intent" /> The shop</label>
				<label class="check"><input type="radio" name="intent" /> Both</label>
				<label class="check"><input type="radio" name="intent" /> Just looking</label>
			</fieldset>
			<div class="row">
				<button class="btn ghost" onclick={back}>Back</button>
				<button class="btn primary" onclick={next}>Continue</button>
			</div>
			<Note from="Beest">The intent question is cheap and tells you who to nudge about the trip.</Note>
		{:else if step === 4}
			<h2>Your first project</h2>
			<p>Do you know what you want to build?</p>
			<div class="row">
				<button class="btn" class:primary={knowsProject === true} onclick={() => (knowsProject = true)}>Yes</button>
				<button class="btn" class:primary={knowsProject === false} onclick={() => (knowsProject = false)}>Not yet</button>
			</div>
			{#if knowsProject === true}
				<label class="field"><span>Project name</span><input type="text" placeholder="My cool project" /></label>
				<label class="field"><span>What does it do? <em>optional</em></span><textarea placeholder="One or two sentences."></textarea></label>
				<label class="field">
					<span>Type</span>
					<select>{#each PROJECT_TYPES as t}<option>{t}</option>{/each}</select>
				</label>
				<button class="btn primary" onclick={next}>Create project</button>
			{:else if knowsProject === false}
				<div class="stack-sm">
					<p class="small">Start with a guide. Each one ends with a shippable project:</p>
					<ul class="small">
						<li>Make a personal website</li>
						<li>Build a game in Godot</li>
						<li>Write a Discord or Slack bot</li>
					</ul>
				</div>
				<div class="row">
					<a class="btn" href="/guides">Browse guides</a>
					<button class="btn primary" onclick={next}>Skip for now</button>
				</div>
			{/if}
			<Note from="Fallen / Horizons">Branches on "do you know what to build?" so beginners aren't stuck at an empty form.</Note>
		{:else}
			<h1>You're in!</h1>
			<p>Your dashboard shows your progress toward the trip. Start building, and ship when it works.</p>
			<a class="btn primary" href="/home">Go to my dashboard</a>
		{/if}
	</div>
</div>
