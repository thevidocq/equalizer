import { useState } from "react";
import { Button, Dialog, Notice, SelectField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useStartScan, type ScanStart } from "../hooks/useSecurity";

/**
 * Starts a scan of one target. The original site is scanned without
 * Equalizer in front: it takes a confirmation, recorded in Activity.
 */
export function ScanButton({ app, target, domain, disabled }: { app: string; target: ScanStart["target"]; domain: string; disabled: boolean }) {
  const start = useStartScan(app);
  const [open, setOpen] = useState(false);
  const [type, setType] = useState<"standard" | "deep">("standard");
  const [sure, setSure] = useState(false);
  const original = target === "original";

  function go() {
    start.mutate({ target, scan_type: type, confirm: original ? sure : undefined }, { onSuccess: () => setOpen(false) });
  }
  return (
    <>
      <Button variant="primary" disabled={disabled} onClick={() => { start.reset(); setSure(false); setOpen(true); }}>Scan now</Button>
      <Dialog open={open} onOpenChange={setOpen} title={original ? "Scan the original site" : "Scan through Equalizer"}
        description={original
          ? `CVE Radar attacks ${domain} directly, without Equalizer in front: what it finds is what the site exposes on its own.`
          : `CVE Radar scans ${domain}, the site as visitors reach it through Equalizer: what it finds is what is left open.`}>
        <div className="flex flex-col gap-4">
          {start.isError ? <Notice tone="error">{describeError(start.error)}</Notice> : null}
          <SelectField label="Kind of scan" value={type} onChange={(e) => setType(e.target.value as "standard" | "deep")}>
            <option value="standard">Standard (a few minutes)</option>
            <option value="deep">Deep (longer, more requests)</option>
          </SelectField>
          {original ? (
            <label className="flex items-start gap-2 text-sm">
              <input type="checkbox" className="mt-1" checked={sure} onChange={(e) => setSure(e.target.checked)} />
              <span>This site is mine, or its owner allowed this scan in writing. My confirmation is recorded in Activity.</span>
            </label>
          ) : null}
          <p className="text-sm text-ink-soft">It counts against your CVE Radar limits (20 scans a day). The results appear here when it ends.</p>
          <div className="flex gap-2">
            <Button variant="primary" busy={start.isPending} disabled={original && !sure} onClick={go}>Start the scan</Button>
            <Button onClick={() => setOpen(false)}>Cancel</Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
