import type { Role } from "../hooks/useUsers";

export const roleText: Record<Role, { name: string; does: string }> = {
  admin: { name: "Administrator", does: "Does everything: sites, maps, connectors, people and settings. Publishes maps alone." },
  viewer: { name: "Viewer", does: "Reads sites, maps, status, activity and logs. Changes nothing." },
};
