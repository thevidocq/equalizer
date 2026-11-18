import { useState } from "react";
import { Button, Notice } from "@/shared/ui";

/** The connector token and the command to run, shown once. */
export function TokenReveal({ connectorId, token }: { connectorId: string; token: string }) {
  const [copied, setCopied] = useState(false);
  const command = [
    "docker run -d --name eq-connector --restart unless-stopped \\",
    "  -e EQ_RELAY_URL=https://<relay domain> \\",
    `  -e EQ_CONNECTOR_ID=${connectorId} \\`,
    `  -e EQ_CONNECTOR_TOKEN=${token} \\`,
    "  -e EQ_ALLOW=<legacy host:port> \\",
    "  ghcr.io/thevidocq/equalizer-connector:latest",
  ].join("\n");

  async function copy() {
    await navigator.clipboard.writeText(command);
    setCopied(true);
  }

  return (
    <div className="flex flex-col gap-3">
      <Notice tone="info">Copy the command now: the token is shown only once. Replace the two values in angle brackets.</Notice>
      <pre className="whitespace-pre-wrap break-all rounded-[var(--radius-control)] bg-ink p-3 font-mono text-[12.5px] leading-relaxed text-paper">{command}</pre>
      <p className="text-sm text-ink-soft">
        <span className="font-semibold text-ink">EQ_ALLOW</span> lists the only addresses the connector may reach, for example the
        legacy server's <span className="font-mono">10.20.0.15:8080</span>.
      </p>
      <div>
        <Button variant="primary" onClick={copy}>{copied ? "Copied" : "Copy command"}</Button>
      </div>
    </div>
  );
}
