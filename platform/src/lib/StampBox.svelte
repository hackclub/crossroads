<script>
	// The stamp box from the Figma ("stamp box", node 219:246): a wooden tray
	// holding the special stamps you haven't stuck on a project yet. It sits
	// in the bottom-right corner of a project page. Drag a stamp out of it
	// onto the project; or click / press Enter on one to stick it on without
	// dragging. The page does the dragging and placing (see ondragstart/onpick).
	import wood from './assets/rail/wood.webp';
	import { effectShort } from './format.js';

	let { stamps = [], open = $bindable(true), ondragstart, onpick } = $props();

	// A slight, steady tilt per stamp, like the Figma's scattered stamps.
	const tilt = (id) => ((id * 37) % 17) - 8;
</script>

<div class="stamp-box-dock">
	{#if open}
		<section class="box" id="stamp-box" aria-label="Your stamp box" style="--wood: url({wood})">
			<div class="tray">
				{#if stamps.length}
					<p class="hint">Drag a stamp onto your project, or pick one to stick it on.</p>
					<ul>
						{#each stamps as s (s.id)}
							<li style="rotate: {tilt(s.id)}deg">
								<button
									type="button"
									class="stamp"
									onpointerdown={(e) => ondragstart?.(s, e)}
									onclick={(e) => {
										// A drag ends in a click too; only a plain click or Enter picks.
										if (e.detail === 0 || !e.currentTarget.dataset.dragged) onpick?.(s);
										delete e.currentTarget.dataset.dragged;
									}}
									aria-label="{s.type.name}: {effectShort(s.type.effect)}. Stick on this project"
								>
									{#if s.type.image_url}<img src={s.type.image_url} alt="" draggable="false" />{:else}<span class="blank">{s.type.name}</span>{/if}
								</button>
								<span class="name">{s.type.name}</span>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="empty">Your stamp box is empty. Open boxes on the <a href="/boxes">Boxes page</a> to get special stamps.</p>
				{/if}
			</div>
		</section>
	{/if}
	<button type="button" class="btn toggle" aria-expanded={open} aria-controls="stamp-box" onclick={() => (open = !open)}>
		{open ? 'Close stamp box' : `Stamp box · ${stamps.length}`}
	</button>
</div>

<style>
	.stamp-box-dock {
		position: fixed;
		right: 24px;
		bottom: 24px;
		z-index: 30;
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 10px;
		text-shadow: none;
	}
	/* Wood under the Figma light leak, 8px corners; the tray sits inset. */
	.box,
	.tray {
		background:
			linear-gradient(rgb(61 46 24 / 0.8), rgb(61 46 24 / 0.8)),
			var(--wood) center / cover;
	}
	.box {
		width: 300px;
		padding: 16px;
		border-radius: 8px;
		box-shadow: 0 6px 12px rgb(0 0 0 / 0.4), 0 24px 60px rgb(0 0 0 / 0.55);
	}
	.tray {
		position: relative;
		max-height: min(480px, 62vh);
		overflow: auto;
		padding: 14px 12px 18px;
		border-radius: 4px;
		box-shadow: inset 0 0 0 999px rgb(0 0 0 / 0.2), inset 0 2px 8px rgb(0 0 0 / 0.5);
	}
	.hint, .empty { margin: 0 0 10px; font-size: 20px; color: rgb(255 255 255 / 0.85); }
	.empty a { color: var(--text); }
	ul { display: flex; flex-wrap: wrap; justify-content: center; gap: 18px 14px; margin: 0; padding: 0; list-style: none; }
	li { display: flex; flex-direction: column; align-items: center; gap: 4px; }
	.stamp {
		padding: 0;
		border: 0;
		background: none;
		cursor: grab;
		touch-action: none; /* the page handles touch drags */
	}
	.stamp:active { cursor: grabbing; }
	.stamp img {
		display: block;
		height: 104px;
		width: auto;
		max-width: 130px;
		filter: drop-shadow(0 3px 6px rgb(0 0 0 / 0.55));
		transition: translate 150ms ease-out;
	}
	.stamp:hover img, .stamp:focus-visible img { translate: 0 -3px; }
	.blank { display: grid; place-items: center; width: 84px; height: 104px; font-size: 18px; border: 2px dashed rgb(255 255 255 / 0.6); }
	.name { font-size: 18px; color: rgb(255 255 255 / 0.85); }
	.toggle { box-shadow: 0 6px 16px rgb(0 0 0 / 0.5); }
	@media (max-width: 820px) {
		.stamp-box-dock { right: 12px; bottom: 12px; }
		.box { width: min(300px, calc(100vw - 24px)); }
		.stamp img { height: 84px; }
	}
	@media (prefers-reduced-motion: reduce) { .stamp img { transition: none; } }
</style>
