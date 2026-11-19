import { Button, SelectField, TextField } from "@/shared/ui";
import type { AuthForm } from "./auth";

type Props = { form: AuthForm; roles: string[]; set: (f: AuthForm) => void };

/** Issuer, client, groups → roles: the part of the login that goes in the map. */
export function AuthFields({ form, roles, set }: Props) {
  const rule = (i: number, k: "group" | "role", v: string) =>
    set({ ...form, roles: form.roles.map((r, n) => (n === i ? { ...r, [k]: v } : r)) });
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 md:grid-cols-2">
        <TextField label="Issuer" value={form.issuer} placeholder="https://login.microsoftonline.com/<tenant id>/v2.0"
          onChange={(e) => set({ ...form, issuer: e.target.value })} hint="From your identity provider: Entra ID, Google, Okta, Keycloak…" />
        <TextField label="Client ID" value={form.client_id} onChange={(e) => set({ ...form, client_id: e.target.value })} />
        <TextField label="Scopes" value={form.scopes} onChange={(e) => set({ ...form, scopes: e.target.value })} hint="Separated by spaces; openid is always added." />
        <TextField label="Groups claim" value={form.groups_claim} onChange={(e) => set({ ...form, groups_claim: e.target.value })}
          hint="Claim of the ID token with the user's groups (Entra ID: groups or roles)." />
      </div>
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 text-sm font-semibold">Who gets which role</legend>
        <p className="text-sm text-ink-soft">The first group the user is in decides. * means every user of the provider. Users in no listed group are refused.</p>
        {form.roles.map((r, i) => (
          <div key={i} className="grid items-end gap-3 sm:grid-cols-[1fr_14rem_auto]">
            <TextField label={`Group ${i + 1}`} value={r.group} onChange={(e) => rule(i, "group", e.target.value)} placeholder="Group name or id, or *" />
            <SelectField label="Role in the map" value={r.role} onChange={(e) => rule(i, "role", e.target.value)}>
              {roles.map((x) => <option key={x} value={x}>{x}</option>)}
            </SelectField>
            <Button variant="quiet" aria-label={`Remove group ${i + 1}`} disabled={form.roles.length === 1}
              onClick={() => set({ ...form, roles: form.roles.filter((_, n) => n !== i) })}>Remove</Button>
          </div>
        ))}
        <div>
          <Button onClick={() => set({ ...form, roles: [...form.roles, { group: "", role: roles[0] ?? "" }] })}>Add a group</Button>
        </div>
      </fieldset>
      <label className="flex items-start gap-2 text-sm">
        <input type="checkbox" className="mt-1" checked={form.required} onChange={(e) => set({ ...form, required: e.target.checked })} />
        <span><span className="font-semibold">Every page needs the login.</span> Off: visitors keep the anonymous role and log in at <code className="font-mono">/.eq/oidc/login</code>.</span>
      </label>
    </div>
  );
}
