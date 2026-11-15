<script>
	// A rich text box for an announcement: what Slack's rich text can show
	// (bold, italic, underline, strikethrough, code, links, lists, quotes and
	// code blocks), and nothing it can't. `content` is Tiptap's JSON, which
	// slack.js turns into a rich_text block. Type :emoji: as you would in Slack.
	import { onMount, onDestroy } from 'svelte';
	import { Editor } from '@tiptap/core';
	import StarterKit from '@tiptap/starter-kit';

	let { content = $bindable(), label = 'Text', editable = true } = $props();

	let element;
	let editor = $state(null);
	// Bumped on every change, so the toolbar shows what's on where the cursor is.
	let tick = $state(0);

	onMount(() => {
		editor = new Editor({
			element,
			extensions: [
				StarterKit.configure({
					// Headings and dividers are their own blocks in Slack.
					heading: false,
					horizontalRule: false,
					link: { openOnClick: false, autolink: true, defaultProtocol: 'https' }
				})
			],
			content: $state.snapshot(content),
			editorProps: { attributes: { class: 'rich-text', 'aria-label': label, 'aria-multiline': 'true', role: 'textbox' } },
			onUpdate: ({ editor }) => (content = editor.getJSON()),
			onTransaction: () => tick++
		});
	});
	onDestroy(() => editor?.destroy());
	// Without emitting an update, which would look like an edit.
	$effect(() => editor?.setEditable(editable, false));

	// Reading tick makes it update with the editor.
	const active = (name) => tick >= 0 && editor?.isActive(name);
	const run = (command) => command(editor.chain().focus()).run();

	function link() {
		const current = editor.getAttributes('link').href ?? '';
		const href = prompt('Link to (leave it empty to remove the link)', current || 'https://');
		if (href === null) return;
		if (!href.trim() || href.trim() === 'https://') run((c) => c.extendMarkRange('link').unsetLink());
		else run((c) => c.extendMarkRange('link').setLink({ href: href.trim() }));
	}

	const TOOLS = [
		['bold', 'B', 'Bold (Ctrl+B)', (c) => c.toggleBold()],
		['italic', 'I', 'Italic (Ctrl+I)', (c) => c.toggleItalic()],
		['underline', 'U', 'Underline (Ctrl+U)', (c) => c.toggleUnderline()],
		['strike', 'S', 'Strikethrough (Ctrl+Shift+S)', (c) => c.toggleStrike()],
		['code', '</>', 'Code (Ctrl+E)', (c) => c.toggleCode()],
		['bulletList', '•', 'Bulleted list', (c) => c.toggleBulletList()],
		['orderedList', '1.', 'Numbered list', (c) => c.toggleOrderedList()],
		['blockquote', '❝', 'Quote', (c) => c.toggleBlockquote()],
		['codeBlock', '{ }', 'Code block', (c) => c.toggleCodeBlock()]
	];
</script>

<div class="rich-box">
	<div class="rich-tools" role="toolbar" aria-label="{label} formatting">
		{#each TOOLS as [name, text, title, command]}
			<button type="button" class="tool tool-{name}" class:on={active(name)} aria-pressed={!!active(name)} {title} disabled={!editor || !editable} onclick={() => run(command)}>{text}</button>
		{/each}
		<button type="button" class="tool" class:on={active('link')} aria-pressed={!!active('link')} title="Link" disabled={!editor || !editable} onclick={link}>Link</button>
	</div>
	<div bind:this={element}></div>
</div>

<style>
	.rich-box { border-radius: 0 0 10px 10px; background: var(--navy-deep); box-shadow: inset 0 0 0 1px var(--navy); }
	.rich-tools { display: flex; flex-wrap: wrap; gap: 4px; padding: 6px 8px; border-bottom: 1px solid var(--navy); }
	.tool { min-width: 34px; padding: 2px 8px; font: inherit; font-size: 20px; color: var(--text); background: transparent; border: 0; border-radius: 6px; cursor: pointer; }
	.tool:hover { background: var(--navy); }
	.tool.on { background: var(--head); color: var(--ink); }
	.tool:disabled { opacity: 0.5; cursor: default; }
	.tool-bold { font-weight: 700; }
	.tool-italic { font-style: italic; }
	.tool-underline { text-decoration: underline; }
	.tool-strike { text-decoration: line-through; }
	.rich-box :global(.rich-text) { min-height: 120px; padding: 10px 16px; font-size: 22px; outline: none; white-space: pre-wrap; }
	.rich-box :global(.rich-text:focus-visible) { box-shadow: inset 0 0 0 2px #fff; border-radius: 0 0 10px 10px; }
	.rich-box :global(.rich-text p) { margin: 0; min-height: 1.3em; }
	.rich-box :global(.rich-text ul), .rich-box :global(.rich-text ol) { margin: 4px 0; padding-left: 28px; }
	.rich-box :global(.rich-text blockquote) { margin: 4px 0; padding-left: 12px; border-left: 4px solid var(--head); }
	.rich-box :global(.rich-text code) { padding: 0 4px; font-family: ui-monospace, monospace; font-size: 0.8em; background: var(--navy); border-radius: 4px; }
	.rich-box :global(.rich-text pre) { margin: 4px 0; padding: 8px 12px; background: var(--navy); border-radius: 6px; }
	.rich-box :global(.rich-text pre code) { background: none; padding: 0; }
	.rich-box :global(.rich-text a) { color: var(--info); }
</style>
