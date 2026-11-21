import { Notice, SelectField, TextField } from "@/shared/ui";
import { useConnectors, useConnectorSetup } from "../hooks/useConnectors";

export type Reach = { mode: "connector" | "direct"; connector_id: string; address: string; scheme: "https" | "http" };

export const emptyReach: Reach = { mode: "connector", connector_id: "", address: "", scheme: "https" };

/** The connector chosen, or the first one: what the form sends. */
export function useChosenConnector(tenant: string, reach: Reach) {
  const connectors = useConnectors(tenant);
  return reach.mode === "direct" ? "" : reach.connector_id || connectors.data?.[0]?.id || "";
}

type Props = { tenant: string; value: Reach; onChange: (r: Reach) => void; hideScheme?: boolean };

/**
 * How Equalizer reaches a site that has no map yet: through a connector
 * installed next to it, or directly at its public address when the site
 * accepts only Equalizer's address.
 */
export function ReachFields({ tenant, value, onChange, hideScheme }: Props) {
  const connectors = useConnectors(tenant);
  const from = useConnectorSetup(tenant).data?.directFrom ?? [];
  const connector = useChosenConnector(tenant, value);
  const set = (patch: Partial<Reach>) => onChange({ ...value, ...patch });
  const direct = value.mode === "direct";

  return (
    <div className="flex flex-col gap-4">
      <SelectField label="How Equalizer reaches the site" value={value.mode}
        onChange={(e) => set({ mode: e.target.value as Reach["mode"] })}>
        <option value="connector">Through a connector installed next to the site</option>
        <option value="direct">Directly, at the site's public address</option>
      </SelectField>
      {direct ? (
        <Notice tone="info">
          The site must accept only Equalizer's {from.length === 1 ? "address" : "addresses"}
          {from.length > 0 ? <>: <span className="font-mono">{from.join(", ")}</span></> : null}. Everyone else must be refused,
          or visitors could skip Equalizer.
        </Notice>
      ) : null}
      {!direct && connectors.data?.length === 0 ? (
        <Notice tone="info">Add a connector first (Connectors in the sidebar), or reach the site directly.</Notice>
      ) : null}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {!direct ? (
          <SelectField label="Connector" value={connector} onChange={(e) => set({ connector_id: e.target.value })}>
            {connectors.data?.map((c) => <option key={c.id} value={c.id}>{c.id}</option>)}
          </SelectField>
        ) : null}
        <TextField label={direct ? "Public address of the site" : "Address on the server"} required
          placeholder={direct ? "203.0.113.10:443" : "127.0.0.1:8080"} value={value.address}
          onChange={(e) => set({ address: e.target.value })}
          hint={direct ? "IP address and port, not a name" : "host:port, as in the connector's EQ_ALLOW"} />
        {hideScheme ? null : (
          <SelectField label="Protocol" value={value.scheme} onChange={(e) => set({ scheme: e.target.value as Reach["scheme"] })}>
            <option value="https">HTTPS</option><option value="http">HTTP</option>
          </SelectField>
        )}
      </div>
      {direct && value.scheme === "http" ? (
        <Notice tone="error">In HTTP the requests cross the Internet in clear: use HTTPS when the site has it.</Notice>
      ) : null}
    </div>
  );
}
