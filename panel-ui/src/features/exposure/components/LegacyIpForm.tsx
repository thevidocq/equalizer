import { useState, type FormEvent } from "react";
import { Button, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useExposureActions } from "../hooks/useExposure";

type Props = { app: string; current: string | null | undefined; neverPublic: boolean };

/**
 * The public IP the site answered on before Equalizer, or "never public"
 * for a site born behind it: then there is nothing to check.
 */
export function LegacyIpForm({ app, current, neverPublic }: Props) {
  const { setIp, setNeverPublic } = useExposureActions(app);
  const toast = useToast();
  const [ip, setValue] = useState(current ?? "");
  const error = setIp.error ?? setNeverPublic.error;

  function submit(e: FormEvent) {
    e.preventDefault();
    setIp.mutate(ip.trim() || null, { onSuccess: () => toast(ip.trim() ? "Old IP saved: the check runs within a minute" : "Checks stopped") });
  }

  function toggleNever(never: boolean) {
    setNeverPublic.mutate(never, { onSuccess: () => toast(never ? "Marked as never public" : "Enter the old public IP to check it") });
  }

  if (neverPublic) {
    return (
      <div className="flex flex-col gap-3">
        {error ? <Notice tone="error">{describeError(error)}</Notice> : null}
        <p className="max-w-prose text-sm text-ink-soft">
          This site never had a public address of its own: it is reachable only through Equalizer, so there is nothing to check.
        </p>
        <div><Button onClick={() => toggleNever(false)} busy={setNeverPublic.isPending}>It had a public address</Button></div>
      </div>
    );
  }
  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      {error ? <Notice tone="error">{describeError(error)}</Notice> : null}
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-full sm:w-72">
          <TextField label="Old public IP of the site" value={ip} onChange={(e) => setValue(e.target.value)}
            placeholder="203.0.113.10" hint="The address its DNS record pointed to before Equalizer. Empty: no checks." />
        </div>
        <Button type="submit" busy={setIp.isPending}>Save</Button>
      </div>
      {!current ? (
        <div>
          <Button onClick={() => toggleNever(true)} busy={setNeverPublic.isPending}>
            The site never had a public address
          </Button>
        </div>
      ) : null}
    </form>
  );
}
