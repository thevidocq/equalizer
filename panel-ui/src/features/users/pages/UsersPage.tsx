import { useState } from "react";
import { useParams } from "react-router";
import { Button, EmptyState, Notice, PageHeader, Table } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import type { User } from "@/shared/api/client";
import { canManage, useMe } from "@/features/auth";
import { useUsers, type Role } from "../hooks/useUsers";
import { AddUserDialog } from "../components/AddUserDialog";
import { roleText } from "../components/roles";

export function UsersPage() {
  const tenant = useParams().tenant ?? "";
  const users = useUsers(tenant);
  const me = useMe();
  const [adding, setAdding] = useState(false);
  const manage = canManage(me.data, tenant);
  const add = manage ? <Button variant="primary" onClick={() => setAdding(true)}>Add a person</Button> : null;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="People" subtitle="Approvals always need two people: an author and someone else." actions={add} />
      {users.isError ? <Notice tone="error">{describeError(users.error)}</Notice> : null}
      {users.data?.length === 0 ? <EmptyState title="Nobody here yet" action={add}>Add at least an editor and an approver.</EmptyState> : null}
      {users.data && users.data.length > 0 ? (
        <Table<User> caption="People" rows={users.data} rowKey={(u) => String(u.id)} columns={[
          { header: "Name", cell: (u) => <span className="font-semibold">{u.name}</span> },
          { header: "Email", cell: (u) => u.email },
          { header: "Role", cell: (u) => (u.tenant_role ? roleText[u.tenant_role as Role].name : "") },
        ]} />
      ) : null}
      <AddUserDialog tenant={tenant} open={adding} onOpenChange={setAdding} />
    </div>
  );
}
