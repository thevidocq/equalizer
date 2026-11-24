import { useState, type FormEvent } from "react";
import { Button, Notice, SelectField, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useSourceActions, type SourceReader, type SourceUpdate } from "../hooks/useSource";

type Method = SourceUpdate["method"];

const HOW: Record<Method, string> = {
  sftp: "The reader logs in over SSH with its own key and copies the code files, read only. Make a user that can only read the site's folder and add the reader's key to its authorized_keys.",
  smb: "The reader opens a Windows share with an account that can only read it. The password is kept encrypted and never shown again.",
  local: "The reader runs on the site's own server and reads the folder there, read only. If the site runs in Docker, name its container: the reader takes its volumes. The command it gives already mounts them.",
};

/** How the source reader reaches the site's files: always read only. */
export function SourceForm({ app, s }: { app: string; s: SourceReader }) {
  const { save } = useSourceActions(app);
  const toast = useToast();
  const [f, setF] = useState({
    method: (s.method ?? "sftp") as Method, host: s.host ?? "", port: s.port ? String(s.port) : "",
    username: s.username ?? "", path: s.path ?? "", share: s.share ?? "", password: "", container: s.container ?? "",
  });
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value } as typeof f);
  const remote = f.method !== "local";

  function submit(e: FormEvent) {
    e.preventDefault();
    const body: SourceUpdate = { method: f.method, path: f.path };
    if (remote) Object.assign(body, { host: f.host, username: f.username, ...(f.port ? { port: Number(f.port) } : {}) });
    if (f.method === "smb") Object.assign(body, { share: f.share, ...(f.password ? { password: f.password } : {}) });
    if (f.method === "local" && f.container.trim()) body.container = f.container.trim();
    save.mutate(body, { onSuccess: () => { setF({ ...f, password: "" }); toast("Saved"); } });
  }

  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-4">
      <h2 className="text-base font-semibold">How the reader reaches the files</h2>
      {save.isError ? <Notice tone="error">{describeError(save.error)}</Notice> : null}
      <SelectField label="Method" value={f.method} onChange={set("method")} hint={HOW[f.method]}>
        <option value="sftp">SFTP (SSH)</option>
        <option value="smb">Windows share (SMB)</option>
        <option value="local">Folder on the same server (Docker too)</option>
      </SelectField>
      {remote ? (
        <div className="grid gap-4 sm:grid-cols-[1fr_8rem]">
          <TextField label="Server" required value={f.host} onChange={set("host")} placeholder="web01.lan or 10.0.0.15" />
          <TextField label="Port" type="number" min={1} max={65535} value={f.port} onChange={set("port")}
            placeholder={f.method === "smb" ? "445" : "22"} />
        </div>
      ) : null}
      {remote ? <TextField label="User" required value={f.username} onChange={set("username")} autoComplete="off" /> : null}
      {f.method === "smb" ? (
        <>
          <TextField label="Share" required value={f.share} onChange={set("share")} placeholder="www" />
          <TextField label="Password" type="password" autoComplete="off" value={f.password} onChange={set("password")}
            required={!s.has_password} placeholder={s.has_password ? "Saved: leave empty to keep it" : ""}
            hint="A saved password serves only the same server and user." />
        </>
      ) : null}
      {f.method === "local" ? (
        <TextField label="Docker container (if the site runs in Docker)" value={f.container} onChange={set("container")} placeholder="wp"
          hint="As docker ps names it. Empty: the folder is on the server itself." />
      ) : null}
      <TextField label="Site folder" required value={f.path} onChange={set("path")}
        placeholder={f.method === "smb" ? "/ (the share's root) or /blog" : "/var/www/html"}
        hint="The folder with the site's code. The reader never opens wp-config.php, .env, keys or uploads." />
      <div>
        <Button type="submit" variant="primary" busy={save.isPending}>Save</Button>
      </div>
    </form>
  );
}
