<script lang="ts">
	import { getStatusClass } from "$lib/utils";

	const { status }: { status: string } = $props();

	const dotColor = $derived(
		status === "online" ? "bg-success"
		: status === "offline" ? "bg-danger"
		: status === "pending" || status === "ip_mismatch" ? "bg-warning"
		: "bg-muted-foreground"
	);
</script>

<span class="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium {getStatusClass(status)}">
	<span class="h-1.5 w-1.5 rounded-full {dotColor}"></span>
	{status}
</span>
