import { useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { Button, Notice, TextField, formatDate } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useAcceptInvitation, useInvitation } from "../hooks/useInvitation";
import { AuthFrame } from "./AuthFrame";

const roleName = { admin: "an administrator", viewer: "a viewer" } as const;

export function InvitePage() {
  const token = useParams().token ?? "";
  const invitation = useInvitation(token);
  const accept = useAcceptInvitation(token);
  const navigate = useNavigate();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const mismatch = confirm !== "" && confirm !== password;

  function submit(e: FormEvent) {
    e.preventDefault();
    if (mismatch) return;
    accept.mutate(password, { onSuccess: () => navigate("/", { replace: true }) });
  }

  if (invitation.isPending) return null;
  if (invitation.isError) {
    return (
      <AuthFrame title="This invitation cannot be used">
        <p className="text-sm text-ink-soft">
          The link was already used, revoked or has expired. Ask your administrator for a new invitation.
        </p>
        <Link to="/login" className="text-sm font-semibold text-signal hover:underline">Go to the login page</Link>
      </AuthFrame>
    );
  }
  const inv = invitation.data;
  return (
    <AuthFrame title={`Join ${inv.tenant_name}`}>
      <p className="text-sm text-ink-soft">
        {inv.name}, you are invited as {roleName[inv.role]}. Choose a password for <strong className="text-ink">{inv.email}</strong>.
        The link expires {formatDate(inv.expires_at)}.
      </p>
      <form onSubmit={submit} className="flex flex-col gap-4">
        {accept.isError ? <Notice tone="error">{describeError(accept.error)}</Notice> : null}
        <input type="email" autoComplete="username" value={inv.email} readOnly hidden />
        <TextField label="Password" type="password" autoComplete="new-password" required minLength={12}
          hint="At least 12 characters." value={password} onChange={(e) => setPassword(e.target.value)} />
        <TextField label="Repeat the password" type="password" autoComplete="new-password" required value={confirm}
          onChange={(e) => setConfirm(e.target.value)} error={mismatch ? "The two passwords differ." : undefined} />
        <Button type="submit" variant="primary" busy={accept.isPending} className="justify-center">Create account</Button>
      </form>
    </AuthFrame>
  );
}
