import { useState } from "react";
import { Button, SelectField, TextField } from "@/shared/ui";
import { mapSnippet, serverSnippet, type Method, type WebServer } from "./snippets";

const methods: Record<Method, { label: string; does: string }> = {
  internal: { label: "Listen only inside (recommended)", does: "The site moves to an internal address that only the connector reaches. The other sites of the server keep their public address." },
  restrict: { label: "Accept only the connector", does: "The site stays on its address but refuses every visitor except the connector. Use it when the site cannot move." },
};

/** How to close the direct access to the site, with the lines to copy. */
export function IsolationGuide({ domain }: { domain: string }) {
  const [method, setMethod] = useState<Method>("internal");
  const [server, setServer] = useState<WebServer>("apache");
  const [input, setInput] = useState({ domain, site: "", internal: "127.0.0.1:8443", connectorIp: "", connector: "", publicAddress: "" });
  const set = (k: keyof typeof input) => (e: { target: { value: string } }) => setInput({ ...input, [k]: e.target.value });
  const config = serverSnippet(server, method, input);
  const map = mapSnippet(method, input, input.connector, input.publicAddress);

  return (
    <section className="flex flex-col gap-4">
      <h2 className="text-base font-semibold">Close the direct access</h2>
      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 text-sm font-semibold">How</legend>
        {(Object.keys(methods) as Method[]).map((m) => (
          <label key={m} className="flex items-start gap-2 text-sm">
            <input type="radio" name="method" className="mt-1" checked={method === m} onChange={() => setMethod(m)} />
            <span><span className="font-semibold">{methods[m].label}.</span> <span className="text-ink-soft">{methods[m].does}</span></span>
          </label>
        ))}
      </fieldset>
      <div className="grid gap-4 sm:grid-cols-3">
        <SelectField label="Web server" value={server} onChange={(e) => setServer(e.target.value as WebServer)}>
          <option value="apache">Apache</option><option value="nginx">nginx</option><option value="iis">IIS</option>
        </SelectField>
        {method === "internal"
          ? <TextField label="Internal address" value={input.internal} onChange={set("internal")} hint="Where the site will listen" />
          : <TextField label="Connector's IP" value={input.connectorIp} onChange={set("connectorIp")} hint="As the web server sees it" />}
        {server === "iis" ? <TextField label="IIS site name" value={input.site} onChange={set("site")} /> : null}
        <TextField label="Connector id" value={input.connector} onChange={set("connector")} hint="From the Connectors page" />
        {method === "restrict" ? <TextField label="Server address" value={input.publicAddress} onChange={set("publicAddress")} hint="host:port the connector connects to" /> : null}
      </div>
      <Snippet title="1. On the web server" text={config} />
      <Snippet title="2. In the site's map" text={map} />
      <p className="text-sm text-ink-soft">
        3. Add the address to the connector's <span className="font-mono">EQ_ALLOW</span>, publish the map, then press <span className="font-semibold">Check now</span>.
      </p>
    </section>
  );
}

function Snippet({ title, text }: { title: string; text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">{title}</h3>
        <Button onClick={async () => { await navigator.clipboard.writeText(text); setCopied(true); }}>{copied ? "Copied" : "Copy"}</Button>
      </div>
      <pre className="overflow-x-auto rounded-[var(--radius-control)] bg-ink p-3 font-mono text-[12.5px] leading-relaxed text-paper">{text}</pre>
    </div>
  );
}
