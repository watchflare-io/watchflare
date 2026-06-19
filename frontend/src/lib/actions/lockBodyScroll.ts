import type { Action } from 'svelte/action';

let lockCount = 0;
let previousOverflow = '';

// Prevents the page behind a modal from scrolling while the node is mounted.
export const lockBodyScroll: Action = () => {
	if (lockCount === 0) {
		previousOverflow = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
	}
	lockCount++;

	return {
		destroy() {
			lockCount--;
			if (lockCount === 0) {
				document.body.style.overflow = previousOverflow;
			}
		}
	};
};
