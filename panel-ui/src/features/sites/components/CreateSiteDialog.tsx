import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { Button, Dialog, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { slugify } from "@/features/tenants";
import { useCreateApp } from "../hooks/useSites";

type Props = { tenant: string; open: boolean; onOpenChange: (o: boolean) => void };

export function CreateSiteDialog({ tenant, open, onOpenChange }: Props) {
  const create = useCreateApp(tenant);
  const toast = useToast();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [domain, setDomain] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(
      { id: slugify(name), name, public_domain: domain.trim().toLowerCase() },
      {
        onSuccess: (a) => {
          toast(`${a.name} added`);
          onOpenChange(false);
          setName("");
          setDomain("");
          navigate(`/t/${tenant}/sites/${a.id}/connectors`);
        },
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Protect a site"
      description="The original site keeps working as it is until you point its domain to the gateway.">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {create.isError ? <Notice tone="error">{describeError(create.error)}</Notice> : null}
        <TextField label="Name" required maxLength={200} value={name} onChange={(e) => setName(e.target.value)}
          hint={name ? `Id: ${slugify(name) || "—"}` : "For example: Permits portal"} />
        <TextField label="Public domain" required placeholder="portal.example.gov" value={domain}
          onChange={(e) => setDomain(e.target.value)} hint="The address visitors type today." />
        <div className="flex justify-end gap-2">
          <Button onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="submit" variant="primary" busy={create.isPending}>Protect site</Button>
        </div>
      </form>
    </Dialog>
  );
}
