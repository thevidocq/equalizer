export type WebServer = "apache" | "nginx" | "iis";
export type Method = "internal" | "restrict";

export type SnippetInput = {
  domain: string;
  site: string;        // IIS site name
  internal: string;    // internal address for "internal", e.g. 127.0.0.1:8443
  connectorIp: string; // address the connector connects from, for "restrict"
};

/** Web server configuration that stops the site answering on its public IP. */
export function serverSnippet(server: WebServer, method: Method, i: SnippetInput): string {
  const [ip, port] = splitAddress(i.internal);
  const from = i.connectorIp || "<connector IP>";
  if (server === "apache") {
    return method === "internal"
      ? `# The site listens only inside: nothing answers for it on the public IP.\nListen ${ip}:${port}\n<VirtualHost ${ip}:${port}>\n    ServerName ${i.domain}\n    # keep the rest of the site's configuration (DocumentRoot, SSL, ...)\n</VirtualHost>\n# and remove ServerName ${i.domain} from the public <VirtualHost *:443> and *:80`
      : `<VirtualHost *:443>\n    ServerName ${i.domain}\n    # only the Equalizer connector may open the site\n    <Location />\n        Require ip ${from}\n    </Location>\n    # keep the rest of the site's configuration\n</VirtualHost>\n# do the same in <VirtualHost *:80> for ${i.domain}`;
  }
  if (server === "nginx") {
    return method === "internal"
      ? `server {\n    # the site listens only inside: nothing answers for it on the public IP\n    listen ${ip}:${port} ssl;\n    server_name ${i.domain};\n    # keep the rest of the site's configuration (root, ssl_certificate, ...)\n}\n# and remove ${i.domain} from the server blocks listening on 443 and 80`
      : `server {\n    listen 443 ssl;\n    server_name ${i.domain};\n    # only the Equalizer connector may open the site\n    allow ${from};\n    deny all;\n    # keep the rest of the site's configuration\n}\n# do the same in the server block on port 80 for ${i.domain}`;
  }
  const name = i.site || "<IIS site name>";
  return method === "internal"
    ? `# PowerShell, as administrator\nImport-Module WebAdministration\nGet-WebBinding -Name "${name}" | Where-Object { $_.bindingInformation -like "*:${i.domain}" } | Remove-WebBinding\nNew-WebBinding -Name "${name}" -Protocol https -IPAddress ${ip} -Port ${port} -HostHeader "${i.domain}" -SslFlags 1`
    : `# PowerShell, as administrator (needs the "IP and Domain Restrictions" feature)\nImport-Module WebAdministration\nAdd-WebConfigurationProperty -PSPath IIS:\\ -Location "${name}" -Filter system.webServer/security/ipSecurity -Name "." -Value @{ipAddress="${from}";allowed="true"}\nSet-WebConfigurationProperty -PSPath IIS:\\ -Location "${name}" -Filter system.webServer/security/ipSecurity -Name allowUnlisted -Value $false`;
}

/** The part of the map that reaches the site the new way. */
export function mapSnippet(method: Method, i: SnippetInput, connector: string, publicAddress: string): string {
  const address = method === "internal" ? i.internal : publicAddress || "<server address>:443";
  return `legacy:\n  base_url: https://${i.domain}\n  reach:\n    mode: connector\n    connector_id: ${connector || "<connector id>"}\n    address: "${address}"\n    allow: ["${address}"]`;
}

function splitAddress(a: string): [string, string] {
  const i = a.lastIndexOf(":");
  if (i <= 0) return ["127.0.0.1", "8443"];
  return [a.slice(0, i), a.slice(i + 1)];
}
