<script lang="ts">
	import { page } from '$app/stores';
	import HostIncidentsSection from '$lib/components/host/HostIncidentsSection.svelte';

	const hostId = $derived($page.params.id);
</script>

<HostIncidentsSection {hostId} />
