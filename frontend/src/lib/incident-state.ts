export type IncidentState = "active" | "paused" | "resolved";

export function incidentState(i: {
	resolved_at: string | null;
	paused_at: string | null;
}): IncidentState {
	if (i.resolved_at) return "resolved";
	if (i.paused_at) return "paused";
	return "active";
}

export const BADGE_CLASSES: Record<IncidentState, string> = {
	active: "bg-destructive/10 text-destructive border border-destructive/20",
	paused: "bg-secondary text-secondary-foreground border border-border",
	resolved: "bg-success/10 text-success border border-success/20",
};

export const BADGE_LABELS: Record<IncidentState, string> = {
	active: "Active",
	paused: "Paused",
	resolved: "Resolved",
};
