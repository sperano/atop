// Mounting components in the browser tests; browser-setup.ts unmounts them
// after each test.
import { flushSync, mount, unmount, type Component } from 'svelte';

const mounted: (() => void)[] = [];

/** Mounts component with props in a new element of the page and returns that element. */
export function render<Props extends Record<string, unknown>>(
	component: Component<Props>,
	props: Props
): HTMLElement {
	const target = document.createElement('div');
	document.body.append(target);
	const instance = mount(component, { target, props });
	flushSync();
	mounted.push(() => {
		void unmount(instance);
		target.remove();
	});
	return target;
}

/** Unmounts everything render mounted. */
export function cleanup(): void {
	for (const dispose of mounted.splice(0)) dispose();
}
