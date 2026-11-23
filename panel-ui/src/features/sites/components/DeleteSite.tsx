import { useState } from "react";
import { useNavigate } from "react-router";
import { Button, Dialog, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useDeleteApp } from "../hooks/useSites";

type Props = { tenant: string; app: string; name: string };

/**
 * Deletes a site for good. Guarded: the administrator types the site name to
 * confirm, because the map, connectors and discoveries go with it.
 */
export function DeleteSite({ tenant, app, name }: Props) {
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState("");
  const del = useDeleteApp(tenant, app);
  const toast = useToast();
  const navigate = useNavigate();

  function confirm() {
    del.mutate(undefined, {
      onSuccess: () => {
        toast(`${name} deleted`);
        setOpen(false);
        navigate(`/t/${tenant}`);
      },
    });
  }

  return (
    <>
      <Button variant="danger" aria-label={`Delete ${name}`} onClick={() => { setTyped(""); setOpen(true); }}>Delete</Button>
      <Dialog open={open} onOpenChange={setOpen} title="Delete this site?"
        description="The map, connectors, discoveries and certificate are removed. The gateway stops serving it. This cannot be undone.">
        <div className="flex flex-col gap-4">
          {del.isError ? <Notice tone="error">{describeError(del.error)}</Notice> : null}
          <TextField label={`Type the site name to confirm`} value={typed} onChange={(e) => setTyped(e.target.value)}
            placeholder={name} hint="Its exact name, as in the list of sites." />
          <div className="flex justify-end gap-2">
            <Button onClick={() => setOpen(false)}>Cancel</Button>
            <Button variant="danger" busy={del.isPending} disabled={typed !== name} onClick={confirm}>Delete site</Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
