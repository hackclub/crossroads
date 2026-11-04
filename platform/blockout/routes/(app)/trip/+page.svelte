<script>
	import HourBar from '#lib/components/HourBar.svelte';
	import Img from '#lib/components/Img.svelte';
	import Note from '#lib/components/Note.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData, PROGRAM } from '#lib/data.js';

	const d = $derived(getData(view.scenario));
	let rsvp = $state(null); // null | 'going' | 'cant'

	const tasks = [
		{ label: 'Confirm you are coming', key: 'rsvp' },
		{ label: 'Parent or guardian consent form', key: 'consent' },
		{ label: 'Passport details (valid until at least July)', key: 'passport' },
		{ label: 'Departure airport and travel dates', key: 'flight' },
		{ label: 'Dietary needs and allergies', key: 'diet' },
		{ label: 'Emergency contact', key: 'emergency' }
	];
	const itinerary = [
		{ day: 'Jan 8', place: 'Paris', text: 'Arrive, meet everyone, opening dinner' },
		{ day: 'Jan 9', place: 'TBD', text: 'Train day: build on board' },
		{ day: 'Jan 10', place: 'TBD', text: 'Explore the city, workshops' },
		{ day: 'Jan 11', place: 'TBD', text: 'Train day, demos' },
		{ day: 'Jan 12', place: 'TBD', text: 'Closing, fly home' }
	];
</script>

<div class="stack-lg">
	<div class="page-head">
		<div class="stack-sm">
			<h1>The trip</h1>
			<p class="muted">{PROGRAM.tripDates} · Starting in {PROGRAM.tripStart} · {PROGRAM.spots} builders · Flights, food and stays covered</p>
		</div>
	</div>

	<Img label="Trip hero photo / map of the route" ratio="21 / 7" />

	{#if !d.qualified}
		<section class="card">
			<h2>Qualify by {PROGRAM.shipDeadline}</h2>
			<HourBar hours={d.hours} />
			<p class="small">Get {PROGRAM.goalHours} hours approved across your shipped projects. Ship before {PROGRAM.shipDeadline} so there's time to review it.</p>
			<div class="row"><a class="btn primary" href="/projects">Go to my projects</a></div>
		</section>
	{:else if rsvp === null}
		<section class="callout good">
			<div class="row between">
				<div class="stack-sm"><strong>You've qualified! 🎉</strong><span class="small">Let us know if you can make it so we can book your travel.</span></div>
				<div class="row">
					<button class="btn primary" onclick={() => (rsvp = 'going')}>I'm coming</button>
					<button class="btn ghost" onclick={() => (rsvp = 'cant')}>I can't make it</button>
				</div>
			</div>
		</section>
	{:else if rsvp === 'cant'}
		<section class="callout info">
			<strong>That's OK.</strong>
			<p class="small">Your approved hours still earn {PROGRAM.currency}, and your spot goes to the next builder. <button class="link-btn" onclick={() => (rsvp = null)}>Changed your mind?</button></p>
		</section>
	{:else}
		<section class="card">
			<div class="card-title"><h2>Before you travel</h2><span class="muted small">1 of {tasks.length} done</span></div>
			<ul class="list">
				{#each tasks as t, i}
					<li>
						<span class={i === 0 ? 'ok' : 'muted'} style="width: 16px">{i === 0 ? '✓' : '○'}</span>
						<span style="flex: 1">{t.label}</span>
						{#if i > 0}<button class="btn small ghost">Fill in</button>{/if}
					</li>
				{/each}
			</ul>
			<Note from="Horizons / Shipwrecked">Each task opens a form. Travel stipend and flight booking details are still to decide.</Note>
		</section>
	{/if}

	<div class="grid-halves">
		<section class="card">
			<h2>Itinerary</h2>
			<ul class="list small">
				{#each itinerary as stop}
					<li><strong style="width: 56px">{stop.day}</strong><span style="flex: 1"><strong>{stop.place}</strong> · {stop.text}</span></li>
				{/each}
			</ul>
			<span class="small muted">Draft. Cities and times will change.</span>
		</section>
		<section class="card">
			<h2>What's covered</h2>
			<ul class="small">
				<li>Flights to Paris and home</li>
				<li>Train tickets between cities</li>
				<li>Hostels / hotels</li>
				<li>All meals</li>
			</ul>
			<h2>Can't travel?</h2>
			<p class="small">Every approved hour still earns {PROGRAM.currency} for the <a href="/shop">shop</a>, including project grants.</p>
			<a class="small" href="/guides#parents">Information for parents</a>
		</section>
	</div>
</div>
