import type { Role } from "../hooks/useUsers";

export const roleText: Record<Role, { name: string; does: string }> = {
  admin: { name: "Administrator", does: "Manages people, sites and connectors, and can approve maps." },
  editor: { name: "Editor", does: "Writes map versions and sends them for review." },
  approver: { name: "Approver", does: "Approves, rejects and publishes versions written by others." },
  auditor: { name: "Auditor", does: "Reads everything, changes nothing." },
};
