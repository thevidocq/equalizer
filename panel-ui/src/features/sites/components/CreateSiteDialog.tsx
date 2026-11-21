import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { Button, Dialog, Notice, SelectField, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { slugify } from "@/features/tenants";
import { useCreateApp, type SiteKind } from "../hooks/useSites";

// Site kinds (docs/compatibility.md): the template of the first map and the agent's profile.
const kinds: { value: string; label: string }[] = [
  { value: "wordpress", label: "WordPress" }, { value: "joomla", label: "Joomla" }, { value: "drupal", label: "Drupal" },
  { value: "magento", label: "Magento / Adobe Commerce" }, { value: "prestashop", label: "PrestaShop" },
  { value: "opencart", label: "OpenCart" }, { value: "typo3", label: "TYPO3" }, { value: "moodle", label: "Moodle" },
  { value: "mediawiki", label: "MediaWiki" }, { value: "phpbb", label: "phpBB" },
  { value: "generic", label: "Another application or a custom site" }, { value: "", label: "I don't know" },
];

type Props = { tenant: string; open: boolean; onOpenChange: (o: boolean) => void };

export function CreateSiteDialog({ tenant, open, onOpenChange }: Props) {
  const create = useCreateApp(tenant);
  const toast = useToast();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [domain, setDomain] = useState("");
  const [kind, setKind] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(
      { id: slugify(name), name, public_domain: domain.trim().toLowerCase(), kind: (kind || null) as SiteKind },
      {
        onSuccess: (a) => {
          toast(`${a.name} added`);
          onOpenChange(false);
          setName("");
          setDomain("");
          setKind("");
          navigate(`/t/${tenant}/sites/${a.id}/discovery`);
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
        <SelectField label="Built with" value={kind} onChange={(e) => setKind(e.target.value)}
          hint="Next, discovery explores the site and proposes a map for you to review. Knowing the software sharpens it.">
          {kinds.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
        </SelectField>
        <div className="flex justify-end gap-2">
          <Button onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="submit" variant="primary" busy={create.isPending}>Protect site</Button>
        </div>
      </form>
    </Dialog>
  );
}
