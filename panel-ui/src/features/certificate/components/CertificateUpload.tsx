import { useState, type FormEvent } from "react";
import { Button, Notice, TextField, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { readCertificateFile, useCertificateActions, type Upload } from "../hooks/useCertificate";

/** Upload the legacy site's own certificate: a .pfx, or PEM files or text. */
export function CertificateUpload({ app, domain, replace }: { app: string; domain: string; replace: boolean }) {
  const { upload } = useCertificateActions(app);
  const toast = useToast();
  const [file, setFile] = useState<File | null>(null);
  const [pem, setPem] = useState("");
  const [password, setPassword] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    const body: Upload = file ? await readCertificateFile(file) : { pem };
    if (password) body.password = password;
    upload.mutate(body, { onSuccess: () => { toast("Certificate saved: the gateways use it within seconds"); setPem(""); setPassword(""); } });
  }

  return (
    <form onSubmit={submit} className="flex max-w-xl flex-col gap-3">
      <div>
        <h2 className="text-base font-semibold">{replace ? "Replace the certificate" : "Upload the certificate"}</h2>
        <p className="mt-1 text-sm text-ink-soft">
          The certificate the site uses today, for {domain}: the same one, not a new one. A .pfx file (IIS exports it this way)
          with its password, or the PEM files of the certificate, its chain and its private key. The key is kept encrypted and
          never shown again; every administrator gets an email when it changes.
        </p>
      </div>
      {upload.isError ? <Notice tone="error">{describeError(upload.error)}</Notice> : null}
      <label className="flex flex-col gap-1.5 text-sm font-medium">
        File (.pfx, .p12, .pem, .crt)
        <input type="file" accept=".pfx,.p12,.pem,.crt,.cer,.key" onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          className="text-sm font-normal" />
      </label>
      {!file ? (
        <label className="flex flex-col gap-1.5 text-sm font-medium">
          Or paste the PEM text (certificate, chain and key)
          <textarea rows={5} value={pem} onChange={(e) => setPem(e.target.value)} spellCheck={false}
            placeholder={"-----BEGIN CERTIFICATE-----\n…\n-----BEGIN PRIVATE KEY-----\n…"}
            className="rounded-[var(--radius-control)] border border-rule bg-surface px-3 py-2 font-mono text-[12px] font-normal" />
        </label>
      ) : null}
      <TextField label="Password (of the .pfx or of an encrypted key)" type="password" autoComplete="off" value={password}
        onChange={(e) => setPassword(e.target.value)} />
      <div><Button type="submit" busy={upload.isPending} disabled={!file && !pem.trim()}>Save the certificate</Button></div>
    </form>
  );
}
