import { useState, type FormEvent } from "react";
import { Button, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useExposureActions } from "../hooks/useExposure";

/** The public IP the site answered on before Equalizer. */
export function LegacyIpForm({ app, current }: { app: string; current: string | null | undefined }) {
  const { setIp } = useExposureActions(app);
  const toast = useToast();
  const [ip, setValue] = useState(current ?? "");

  function submit(e: FormEvent) {
    e.preventDefault();
    setIp.mutate(ip.trim() || null, { onSuccess: () => toast(ip.trim() ? "Old IP saved: the check runs within a minute" : "Checks stopped") });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      {setIp.isError ? <Notice tone="error">{describeError(setIp.error)}</Notice> : null}
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-72">
          <TextField label="Old public IP of the site" value={ip} onChange={(e) => setValue(e.target.value)}
            placeholder="203.0.113.10" hint="The address its DNS record pointed to. Empty: no checks." />
        </div>
        <Button type="submit" busy={setIp.isPending}>Save</Button>
      </div>
    </form>
  );
}
