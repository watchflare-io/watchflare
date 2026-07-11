import { Home, Server, Package, AlertCircle, Box } from 'lucide-svelte';

export const navItems = [
	{ href: '/', label: 'Dashboard', icon: Home },
	{ href: '/hosts', label: 'Hosts', icon: Server },
	{ href: '/containers', label: 'Containers', icon: Box },
	{ href: '/incidents', label: 'Incidents', icon: AlertCircle },
	{ href: '/packages', label: 'Packages', icon: Package }
];

export const settingsItems = [
	{ href: '/settings', label: 'General' },
	{ href: '/settings/notifications', label: 'Notifications' }
];
