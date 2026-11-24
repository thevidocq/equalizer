import { useState, type FormEvent } from "react";
import { Button, Dialog, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useRenameApp } from "../hooks/useSites";

const icon = "inline-flex size-7 items-center justify-center rounded-md text-ink-soft transition-colors hover:bg-ink/8 hover:text-ink";

/** The pencil next to a site's name: renames it; address, domain and map stay the same. */
export function RenameSite({ tenant, app, name }: { tenant: string; app: string; name: string }) {
  const rename = useRenameApp(tenant, app);
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState(name);

  function submit(e: FormEvent) {
    e.preventDefault();
    rename.mutate(value, { onSuccess: () => { setOpen(false); toast("Site renamed"); } });
  }
  return (
    <>
      <button type="button" className={icon} aria-label="Rename the site" title="Rename"
        onClick={() => { setValue(name); rename.reset(); setOpen(true); }}>
        <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M4 20h4L19 9a2.1 2.1 0 0 0-4-4L4 16v4z" />
          <path d="M13.5 6.5l4 4" />
        </svg>
      </button>
      <Dialog open={open} onOpenChange={setOpen} title="Rename the site"
        description="Only the name shown in the panel and in emails changes: the domain, the address in the panel and the map stay the same.">
        <form onSubmit={submit} className="flex flex-col gap-4">
          {rename.isError ? <Notice tone="error">{describeError(rename.error)}</Notice> : null}
          <TextField label="Name" required maxLength={200} value={value} onChange={(e) => setValue(e.target.value)} autoFocus />
          <div className="flex gap-2">
            <Button type="submit" variant="primary" busy={rename.isPending} disabled={!value.trim() || value.trim() === name}>Save</Button>
            <Button onClick={() => setOpen(false)}>Cancel</Button>
          </div>
        </form>
      </Dialog>
    </>
  );
}
