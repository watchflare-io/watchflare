import type { Action } from 'svelte/action';

// Focuses the node when it mounts. Replacement for the autofocus attribute,
// which triggers Svelte's a11y_autofocus warning.
export const autofocus: Action<HTMLElement> = (node) => {
	node.focus();
};
