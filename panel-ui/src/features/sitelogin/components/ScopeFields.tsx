import { Button } from "@/shared/ui";
import type { AuthForm } from "./auth";
import { loginPresets, presetNames } from "./presets";

type Props = { form: AuthForm; kind?: string | null; set: (f: AuthForm) => void };

const choices: { id: AuthForm["scope"]; title: string; text: string }[] = [
  { id: "site", title: "The whole site", text: "Every page asks for the login first. For intranets and internal applications." },
  { id: "paths", title: "Only some paths", text: "Admin and login pages ask for it; the public site stays open to everyone." },
  { id: "optional", title: "Optional", text: "Nobody is sent to the login; visitors log in at /.eq/oidc/login." },
];

/** Where the identity provider's login is asked. */
export function ScopeFields({ form, kind, set }: Props) {
  const preset = kind ? loginPresets[kind] : undefined;
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="mb-1 text-sm font-semibold">Where the login is asked</legend>
      {choices.map((c) => (
        <label key={c.id} className="flex items-start gap-2 text-sm">
          <input type="radio" name="scope" className="mt-1" checked={form.scope === c.id} onChange={() => set({ ...form, scope: c.id })} />
          <span><span className="font-semibold">{c.title}.</span> {c.text}</span>
        </label>
      ))}
      {form.scope === "paths" ? (
        <div className="ml-6 flex flex-col gap-2">
          <label className="flex flex-col gap-1.5 text-sm font-medium">
            Paths that ask for the login
            <textarea
              rows={5} value={form.paths} onChange={(e) => set({ ...form, paths: e.target.value })}
              placeholder={"/wp-admin/\n/wp-login.php"}
              className="scroll-box max-w-md rounded-[var(--radius-control)] border border-rule bg-surface px-3 py-2 font-mono text-[13px]"
            />
            <span className="font-normal text-ink-soft">
              One per line; each covers what starts with it. Upper case, double slashes and similar tricks are matched too.
            </span>
          </label>
          {preset ? (
            <div>
              <Button onClick={() => set({ ...form, paths: preset.join("\n") })}>
                Use the {presetNames[kind!]} paths
              </Button>
            </div>
          ) : null}
          <p className="max-w-prose text-sm text-ink-soft">
            Until a visitor passes the identity provider, the site&apos;s own HTTP login (e.g. WordPress application
            passwords) never reaches the site.
          </p>
        </div>
      ) : null}
    </fieldset>
  );
}
