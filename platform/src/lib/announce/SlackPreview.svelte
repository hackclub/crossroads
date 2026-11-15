<script>
	// Roughly how Slack will show an announcement: the blocks slack.js built,
	// under the poster's name with the APP tag. Custom emoji show as their
	// :names: here; "Send test to me" shows the real thing.
	let { blocks, name = 'You', channel = '' } = $props();

	// Common emoji, so the preview isn't all :names:. Slack has the rest.
	const EMOJI = {
		tada: '🎉', train: '🚆', steam_locomotive: '🚂', railway_car: '🚃', station: '🚉', ticket: '🎫', admission_tickets: '🎟️',
		wave: '👋', fire: '🔥', sparkles: '✨', eyes: '👀', heart: '❤️', rocket: '🚀', star: '⭐', star2: '🌟', zap: '⚡',
		white_check_mark: '✅', x: '❌', warning: '⚠️', point_right: '👉', point_down: '👇', raised_hands: '🙌', clap: '👏',
		thumbsup: '👍', '+1': '👍', pray: '🙏', smile: '😄', joy: '😂', sob: '😭', shocked: '😮', partying_face: '🥳',
		gift: '🎁', moneybag: '💰', calendar: '📅', alarm_clock: '⏰', hourglass: '⌛', mega: '📣', loudspeaker: '📢',
		pushpin: '📌', memo: '📝', computer: '💻', earth_africa: '🌍', earth_americas: '🌎', snowflake: '❄️', mountain: '⛰️',
		flag_fr: '🇫🇷', flag_de: '🇩🇪', flag_ch: '🇨🇭', flag_be: '🇧🇪', flag_nl: '🇳🇱', flag_it: '🇮🇹', flag_at: '🇦🇹', flag_eu: '🇪🇺'
	};

	const listStart = (list) => (list.offset ?? 0) + 1;
	const styleClass = (style) => (style ? Object.keys(style).filter((k) => style[k]).map((k) => 's-' + k).join(' ') : '');
</script>

{#snippet inline(elements)}
	{#each elements as el}
		{#if el.type === 'text'}<span class={styleClass(el.style)}>{el.text}</span>
		{:else if el.type === 'link'}<a class={styleClass(el.style)} href={el.url} target="_blank" rel="noreferrer">{el.text || el.url}</a>
		{:else if el.type === 'emoji'}{#if EMOJI[el.name]}<span class="emoji" title=":{el.name}:">{EMOJI[el.name]}</span>{:else}<span class="emoji-name">:{el.name}:</span>{/if}
		{:else if el.type === 'broadcast'}<span class="mention">@{el.range}</span>
		{/if}
	{/each}
{/snippet}

<div class="slack" aria-label="Preview of the Slack message">
	{#if channel}<div class="channel"># {channel}</div>{/if}
	<div class="message">
		<div class="avatar" aria-hidden="true">{name.slice(0, 1).toUpperCase()}</div>
		<div class="body">
			<div class="meta"><b>{name}</b> <span class="app">APP</span> <span class="time">now</span></div>
			{#each blocks as block}
				{#if block.type === 'rich_text'}
					<div class="rich">
						{#each block.elements as part}
							{#if part.type === 'rich_text_section'}<div class="section">{@render inline(part.elements)}</div>
							{:else if part.type === 'rich_text_list'}
								<svelte:element this={part.style === 'ordered' ? 'ol' : 'ul'} class="slist" style:margin-left="{part.indent * 22}px" start={part.style === 'ordered' ? listStart(part) : undefined} class:bullet-2={part.indent % 3 === 1} class:bullet-3={part.indent % 3 === 2}>
									{#each part.elements as item}<li>{@render inline(item.elements)}</li>{/each}
								</svelte:element>
							{:else if part.type === 'rich_text_quote'}<blockquote>{@render inline(part.elements)}</blockquote>
							{:else if part.type === 'rich_text_preformatted'}<pre>{part.elements.map((e) => e.text).join('')}</pre>
							{/if}
						{/each}
					</div>
				{:else if block.type === 'header'}<div class="header">{block.text.text}</div>
				{:else if block.type === 'image'}
					<figure>
						{#if block.title}<figcaption>{block.title.text}</figcaption>{/if}
						<img src={block.image_url} alt={block.alt_text} />
					</figure>
				{:else if block.type === 'divider'}<hr />
				{:else if block.type === 'actions'}
					<div class="actions">
						{#each block.elements as button}
							<a class="button {button.style ?? ''}" href={button.url} target="_blank" rel="noreferrer">{button.text.text}</a>
						{/each}
					</div>
				{/if}
			{:else}
				<p class="empty">Nothing to show yet.</p>
			{/each}
		</div>
	</div>
</div>

<style>
	/* Slack's dark theme, in Slack's own type rather than the platform's. */
	.slack {
		font-family: Lato, 'Helvetica Neue', Helvetica, Arial, sans-serif;
		font-size-adjust: none;
		font-size: 15px;
		line-height: 1.47;
		color: #d1d2d3;
		background: #1a1d21;
		border: 1px solid #35373b;
		border-radius: 8px;
		overflow: hidden;
		text-shadow: none;
	}
	.channel { padding: 8px 16px; font-weight: 700; color: #fff; border-bottom: 1px solid #35373b; }
	.message { display: flex; gap: 8px; padding: 12px 16px; }
	.avatar { flex: none; display: grid; place-items: center; width: 36px; height: 36px; font-weight: 700; color: #fff; background: #4a154b; border-radius: 6px; }
	.body { flex: 1; min-width: 0; }
	.meta { display: flex; align-items: baseline; gap: 6px; }
	.meta b { color: #fff; font-weight: 900; }
	.app { padding: 0 3px; font-size: 10px; font-weight: 700; color: #ababad; background: #35373b; border-radius: 2px; }
	.time { font-size: 12px; color: #ababad; }
	.rich, .header, figure, hr, .actions { margin-top: 4px; }
	.section { white-space: pre-wrap; overflow-wrap: anywhere; }
	.slist { margin-block: 0; padding-left: 26px; white-space: pre-wrap; }
	ul.slist { list-style: disc; }
	ul.slist.bullet-2 { list-style: circle; }
	ul.slist.bullet-3 { list-style: square; }
	blockquote { margin: 0; padding-left: 12px; border-left: 4px solid #545454; white-space: pre-wrap; }
	pre { margin: 4px 0; padding: 8px; font-family: Monaco, Menlo, Consolas, monospace; font-size: 12px; white-space: pre-wrap; background: #222529; border: 1px solid #35373b; border-radius: 4px; }
	.s-bold { font-weight: 700; }
	.s-italic { font-style: italic; }
	.s-underline { text-decoration: underline; }
	.s-strike { text-decoration: line-through; }
	.s-underline.s-strike { text-decoration: underline line-through; }
	.s-code { padding: 2px 3px; font-family: Monaco, Menlo, Consolas, monospace; font-size: 12px; color: #e8912d; background: #222529; border: 1px solid #35373b; border-radius: 3px; }
	a { color: #1d9bd1; text-decoration: none; }
	a:hover { text-decoration: underline; }
	.emoji { font-size: 1.15em; }
	.emoji-name { padding: 0 3px; font-size: 13px; color: #ababad; background: #222529; border-radius: 4px; }
	.mention { padding: 0 2px; color: #f2c744; background: rgb(242 199 68 / 0.2); border-radius: 3px; }
	.header { font-size: 18px; font-weight: 900; color: #fff; }
	figure { margin-inline: 0; margin-bottom: 0; }
	figcaption { font-size: 13px; color: #ababad; }
	figure img { display: block; max-width: 100%; max-height: 360px; border-radius: 6px; }
	hr { border: 0; border-top: 1px solid #35373b; }
	.actions { display: flex; flex-wrap: wrap; gap: 8px; }
	.button { padding: 3px 12px; font-size: 14px; font-weight: 700; color: #d1d2d3; border: 1px solid #565856; border-radius: 4px; }
	.button:hover { text-decoration: none; background: #222529; }
	.button.primary { color: #fff; background: #007a5a; border-color: #007a5a; }
	.button.danger { color: #fff; background: #e01e5a; border-color: #e01e5a; }
	.empty { margin: 4px 0 0; color: #ababad; font-style: italic; }
</style>
