import { useState, type FormEvent } from "react";
import { Button, IntegrationCard, Notice, TextField, useToast, type IntegrationState } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useScanner, useScannerActions } from "../hooks/useScanner";

/**
 * The tenant's vulnerability scanner (CVE Radar): the address of its API and
 * a key. The panel tries them before saving; the key is never shown again.
 */
export function ScannerForm({ tenant }: { tenant: string }) {
  const current = useScanner(tenant);
  if (current.isError) return <Notice tone="error">{describeError(current.error)}</Notice>;
  if (!current.data) return null;
  return <Form key={current.data.endpoint ?? "none"} tenant={tenant} endpoint={current.data.endpoint ?? ""}
    hasKey={current.data.has_api_key} hint={current.data.key_hint ?? null} status={current.data.status} />;
}

const STATE: Record<string, [IntegrationState, string]> = {
  connected: ["connected", "Connected"], not_connected: ["not_connected", "Not connected"], key_refused: ["error", "Key refused"],
};

type FormProps = { tenant: string; endpoint: string; hasKey: boolean; hint: string | null; status: string };

function Form({ tenant, endpoint: saved, hasKey, hint, status }: FormProps) {
  const { save, remove } = useScannerActions(tenant);
  const toast = useToast();
  const [endpoint, setEndpoint] = useState(saved);
  const [key, setKey] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    save.mutate({ endpoint, api_key: key || undefined }, {
      onSuccess: () => { setKey(""); toast("Scanner connected: the address and key work"); },
    });
  }
  const error = save.error ?? remove.error;
  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-4">
      <h2 className="text-base font-semibold">Security scanner</h2>
      <IntegrationCard logo="/brands/cve-radar.png" name="CVE Radar" description="Vulnerability scans of your sites, read against their maps"
        state={(STATE[status] ?? STATE.not_connected)[0]} label={(STATE[status] ?? STATE.not_connected)[1]}
        note={status === "key_refused" ? "CVE Radar refused the saved key at the last call (revoked, expired or without permissions): save a new one." : undefined} />
      <p className="text-sm text-ink-soft">
        Create the key in CVE Radar (API section) with the read, scan and write permissions. The panel tries the address and the
        key before saving, keeps the key encrypted and never shows it again.
      </p>
      {error ? <Notice tone="error">{describeError(error)}</Notice> : null}
      <TextField label="API address" type="url" required value={endpoint} placeholder="https://…/api/public/v1"
        onChange={(e) => setEndpoint(e.target.value)} hint="The base address of CVE Radar's API, from its documentation." />
      <TextField label="API key" type="password" autoComplete="off" value={key} onChange={(e) => setKey(e.target.value)}
        required={!hasKey} placeholder={hasKey ? `Saved${hint ? ` (…${hint})` : ""}: leave empty to keep it` : "cvr_live_… or cvr_test_…"} />
      <div className="flex flex-wrap gap-2">
        <Button type="submit" variant="primary" busy={save.isPending}>Save</Button>
        {hasKey ? (
          <Button variant="danger" busy={remove.isPending} onClick={() => remove.mutate(undefined, { onSuccess: () => toast("Scanner disconnected") })}>
            Disconnect
          </Button>
        ) : null}
      </div>
    </form>
  );
}
