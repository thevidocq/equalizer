import { useState } from "react";
import { Button, Notice, formatDate, useToast } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useSourceActions, type SourceReader } from "../hooks/useSource";

const STATUS: Record<SourceReader["status"], string> = {
  not_set_up: "Not set up",
  not_installed: "Not installed: start it with a token",
  waiting: "Not answering: is its container running?",
  ready: "Ready",
};

/** Whether the reader runs, its key for SFTP, and its token with the command that starts it. */
export function ReaderStatus({ app, s, isAdmin }: { app: string; s: SourceReader; isAdmin: boolean }) {
  const { token } = useSourceActions(app);
  const toast = useToast();
  const [copied, setCopied] = useState("");
  async function copy(what: string, text: string) {
    await navigator.clipboard.writeText(text);
    setCopied(what);
  }
  const install = token.data?.install;
  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-base font-semibold">The reader</h2>
      <p className="text-sm">
        <span className="font-semibold">{STATUS[s.status]}</span>
        {s.last_seen ? <span className="text-ink-soft"> · last heard {formatDate(s.last_seen)}{s.version ? ` · version ${s.version}` : ""}</span> : null}
      </p>
      {s.method === "sftp" && s.public_key ? (
        <div className="flex flex-col gap-2">
          <p className="text-sm text-ink-soft">The reader's key: add this line to the read-only user's <span className="font-mono">~/.ssh/authorized_keys</span>.</p>
          <pre className="whitespace-pre-wrap break-all rounded-[var(--radius-control)] bg-ink p-3 font-mono text-[12.5px] text-paper">{s.public_key}</pre>
          <div><Button onClick={() => copy("key", s.public_key ?? "")}>{copied === "key" ? "Copied" : "Copy key"}</Button></div>
        </div>
      ) : null}
      {isAdmin ? (
        <>
          {token.isError ? <Notice tone="error">{describeError(token.error)}</Notice> : null}
          {install ? (
            <div className="flex flex-col gap-2">
              <Notice tone="info">Copy the command now: the token is shown only once. Run it next to the connector; with a mounted folder or a Docker container, on the site's server. It mounts the site read only. If you change the settings above, make a new token: the command follows them.</Notice>
              <pre className="whitespace-pre-wrap break-all rounded-[var(--radius-control)] bg-ink p-3 font-mono text-[12.5px] leading-relaxed text-paper">{install}</pre>
              <div><Button variant="primary" onClick={() => copy("cmd", install)}>{copied === "cmd" ? "Copied" : "Copy command"}</Button></div>
            </div>
          ) : (
            <div>
              <Button busy={token.isPending} disabled={!s.configured} onClick={() => token.mutate(undefined, { onSuccess: () => toast("New token made: the old one stopped") })}>
                {s.status === "not_set_up" || s.status === "not_installed" ? "Make token and command" : "Make a new token"}
              </Button>
            </div>
          )}
        </>
      ) : null}
    </section>
  );
}
