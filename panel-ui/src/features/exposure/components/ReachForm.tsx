import { useState, type FormEvent } from "react";
import type { App } from "@/shared/api/client";
import { Button, Notice, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { ReachFields, useChosenConnector, type Reach } from "@/features/connectors";
import { useExposureActions } from "../hooks/useExposure";

/** How Equalizer reaches this site. Changing it republishes the current map,
 * keeping its tuned entries. Shown once a map is published. */
export function ReachForm({ tenant, site }: { tenant: string; site: App }) {
  const current: Reach = {
    mode: site.reach_mode === "direct" ? "direct" : "connector",
    connector_id: site.reach_connector_id ?? "",
    address: site.reach_address ?? "",
    scheme: "https",
  };
  const [reach, setReach] = useState<Reach>(current);
  const connector = useChosenConnector(tenant, reach);
  const { setReach: save } = useExposureActions(site.id);
  const toast = useToast();
  const changed = reach.mode !== current.mode || reach.address.trim() !== current.address ||
    (reach.mode === "connector" && connector !== current.connector_id);

  function submit(e: FormEvent) {
    e.preventDefault();
    save.mutate(
      { mode: reach.mode, connector_id: reach.mode === "connector" ? connector : undefined, address: reach.address.trim() },
      { onSuccess: () => toast("Reach saved, map republished") },
    );
  }
  return (
    <form onSubmit={submit} className="flex flex-col gap-4 border border-rule bg-surface p-5">
      <div>
        <h2 className="text-base font-semibold">How Equalizer reaches this site</h2>
        <p className="text-sm text-ink-soft">Change it anytime. The current map is republished and your tuned entries are kept.</p>
      </div>
      {save.isError ? <Notice tone="error">{describeError(save.error)}</Notice> : null}
      <ReachFields tenant={tenant} value={reach} onChange={setReach} hideScheme />
      <div>
        <Button type="submit" variant="primary" busy={save.isPending}
          disabled={!changed || !reach.address.trim() || (reach.mode === "connector" && !connector)}>
          Save and republish
        </Button>
      </div>
    </form>
  );
}
