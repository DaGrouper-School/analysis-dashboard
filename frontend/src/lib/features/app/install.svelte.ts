/** The browser's install prompt (Chrome, Edge, Android). Not in the DOM typings yet. */
interface InstallPromptEvent extends Event {
	prompt(): Promise<void>;
	userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

function runningInstalled(): boolean {
	if (typeof matchMedia === 'undefined') return false;
	return (
		matchMedia('(display-mode: standalone)').matches ||
		(navigator as Navigator & { standalone?: boolean }).standalone === true
	);
}

/**
 * Whether the app can be installed from a button, and whether it already runs installed.
 * Safari on iPhone and iPad has no prompt: people add it from the Share menu instead.
 */
export const install = $state({
	prompt: null as InstallPromptEvent | null,
	installed: runningInstalled(),
	ios:
		typeof navigator !== 'undefined' &&
		/iPad|iPhone|iPod/.test(navigator.userAgent) &&
		!/CriOS|FxiOS|EdgiOS/.test(navigator.userAgent)
});

/**
 * Listens for the install prompt and for the app being installed. The browser offers the prompt
 * once, early, so the root layout starts listening on every page. Returns the cleanup.
 */
export function watchInstall(): () => void {
	const onPrompt = (event: Event) => {
		event.preventDefault();
		install.prompt = event as InstallPromptEvent;
	};
	const onInstalled = () => {
		install.prompt = null;
		install.installed = true;
	};
	addEventListener('beforeinstallprompt', onPrompt);
	addEventListener('appinstalled', onInstalled);
	return () => {
		removeEventListener('beforeinstallprompt', onPrompt);
		removeEventListener('appinstalled', onInstalled);
	};
}

/** Shows the browser's install dialog. The prompt can be used once. */
export async function promptInstall(): Promise<void> {
	const prompt = install.prompt;
	if (!prompt) return;
	install.prompt = null;
	await prompt.prompt();
	if ((await prompt.userChoice).outcome === 'accepted') install.installed = true;
}
