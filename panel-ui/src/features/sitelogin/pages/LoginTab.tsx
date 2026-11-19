import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { hasRole, useMe } from "@/features/auth";
import { useCreateDraft, useMapVersion, useMapVersions } from "@/features/maps";
import { describeError } from "@/shared/api/errors";
import { Button, EmptyState, Notice } from "@/shared/ui";
import { applyAuth, problems, readAuth, rolesOf, type AuthForm } from "../components/auth";
import { AuthFields } from "../components/AuthFields";
import { SecretForm } from "../components/SecretForm";
import { useOIDCInfo } from "../hooks/useSiteLogin";

/** Login with the organisation's identity provider in front of the site. Off by default. */
export function LoginTab() {
  const { tenant = "", app = "" } = useParams();
  const me = useMe().data;
  const navigate = useNavigate();
  const versions = useMapVersions(app).data ?? [];
  const source = versions.find((v) => v.status === "published") ?? versions[0];
  const base = useMapVersion(app, source?.version ?? 0).data;
  const doc = base?.document as Record<string, unknown> | undefined;
  const info = useOIDCInfo(app).data;
  const create = useCreateDraft(app);
  const [form, setForm] = useState<AuthForm>(() => readAuth(undefined));
  useEffect(() => { if (doc) setForm(readAuth(doc)); }, [doc]);
  const roles = useMemo(() => rolesOf(doc), [doc]);
  const canWrite = hasRole(me, tenant, "admin", "editor");
  const isAdmin = hasRole(me, tenant, "admin");
  const issues = problems(form, doc);
  const live = source?.status === "published" && readAuth(doc).on;

  if (!versions.length) return <EmptyState title="No map yet">The login is set in the map: write the first version, then come back here.</EmptyState>;

  function save() {
    if (!doc || !source) return;
    const note = form.on ? "Login with the identity provider" : "Login with the identity provider turned off";
    create.mutate({ document: applyAuth(doc, form), note },
      { onSuccess: (v) => navigate(`/t/${tenant}/sites/${app}/maps/${v.version}`) });
  }

  return (
    <div className="flex max-w-4xl flex-col gap-6">
      <p className="text-sm">
        {live ? "Visitors log in with your identity provider before they reach the site."
          : "Off: visitors reach the site with its own login, as today."}{" "}
        The settings below go in a new version of the map, reviewed and approved like any other; the secret is kept apart.
        Starting from version {source?.version} ({source?.status}).
      </p>
      {create.isError ? <Notice tone="error">{describeError(create.error)}</Notice> : null}
      <label className="flex items-center gap-2 text-sm font-semibold">
        <input type="checkbox" checked={form.on} disabled={!canWrite} onChange={(e) => setForm({ ...form, on: e.target.checked })} />
        Log visitors in with the identity provider (OIDC)
      </label>
      {form.on ? <AuthFields form={form} roles={roles} set={setForm} /> : null}
      {issues.length ? <Notice tone="info">{issues.join(" ")}</Notice> : null}
      {canWrite ? (
        <div><Button variant="primary" busy={create.isPending} disabled={!doc || issues.length > 0} onClick={save}>Save as new draft</Button></div>
      ) : null}
      {form.on && info ? (
        <section className="flex flex-col gap-3 border-t border-rule-soft pt-5">
          <h3 className="text-base font-semibold">In your identity provider</h3>
          <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-4 gap-y-1 text-sm">
            <dt className="text-ink-soft">Redirect URI</dt><dd className="break-all font-mono">{info.redirect_uri}</dd>
            <dt className="text-ink-soft">After logout</dt><dd className="break-all font-mono">{info.logout_uri}</dd>
          </dl>
          <SecretForm app={app} has={info.has_secret} canEdit={isAdmin} />
        </section>
      ) : null}
    </div>
  );
}
