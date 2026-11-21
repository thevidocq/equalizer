import { useState } from "react";
import { Button, Notice } from "@/shared/ui";

/** The connector token and the command to run, shown once. */
type Props = { connectorId: string; token: string; version?: string; relayUrl?: string };

export function TokenReveal({ connectorId, token, version, relayUrl }: Props) {
  // the release of the platform; a development build has none: latest
  const tag = version && /^v?\d+\.\d+\.\d+$/.test(version) ? version.replace(/^v/, "") : "latest";
  const [copied, setCopied] = useState(false);
  const command = [
    "docker run -d --name eq-connector --restart unless-stopped --network host \\",
    `  -e EQ_RELAY_URL=${relayUrl || "https://<relay domain>"} \\`,
    `  -e EQ_CONNECTOR_ID=${connectorId} \\`,
    `  -e EQ_CONNECTOR_TOKEN=${token} \\`,
    "  -e EQ_ALLOW=<addresses, e.g. 127.0.0.1:8080,10.20.0.15:443> \\",
    `  ghcr.io/thevidocq/equalizer-connector:${tag}`,
  ].join("\n");

  async function copy() {
    await navigator.clipboard.writeText(command);
    setCopied(true);
  }

  return (
    <div className="flex flex-col gap-3">
      <Notice tone="info">
        Copy the command now: the token is shown only once. Replace {relayUrl ? "the value" : "the values"} in angle brackets.
      </Notice>
      <pre className="whitespace-pre-wrap break-all rounded-[var(--radius-control)] bg-ink p-3 font-mono text-[12.5px] leading-relaxed text-paper">{command}</pre>
      <p className="text-sm text-ink-soft">
        <span className="font-semibold text-ink">EQ_ALLOW</span> lists the only addresses the connector may reach: put every address
        your protected sites listen on. Sites sharing an address are told apart by name (virtual hosts), so one entry can serve
        several sites. <span className="font-mono">--network host</span> lets it reach sites that listen only on this server's
        <span className="font-mono"> 127.0.0.1</span>.
      </p>
      <div>
        <Button variant="primary" onClick={copy}>{copied ? "Copied" : "Copy command"}</Button>
      </div>
    </div>
  );
}
