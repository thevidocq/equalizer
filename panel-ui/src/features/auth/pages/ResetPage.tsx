import { useState, type FormEvent } from "react";
import { Link, useParams } from "react-router";
import { Button, Notice, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useFinishReset, useResetLink } from "../hooks/useReset";
import { AuthFrame } from "./AuthFrame";

export function ResetPage() {
  const token = useParams().token ?? "";
  const link = useResetLink(token);
  const finish = useFinishReset(token);
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const mismatch = confirm !== "" && confirm !== password;

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!mismatch) finish.mutate(password);
  }

  if (link.isPending) return null;
  if (finish.isSuccess) {
    return (
      <AuthFrame title="Password changed">
        <p className="text-sm text-ink-soft">Log in with the new password; the panel still asks for the code of your authenticator app.</p>
        <Link to="/login" className="text-sm font-semibold text-signal hover:underline">Go to the login</Link>
      </AuthFrame>
    );
  }
  if (link.isError) {
    return (
      <AuthFrame title="This link cannot be used">
        <p className="text-sm text-ink-soft">It was already used or has expired (links work for one hour). Ask for a new one.</p>
        <Link to="/forgot" className="text-sm font-semibold text-signal hover:underline">Ask for a new link</Link>
      </AuthFrame>
    );
  }
  return (
    <AuthFrame title="Choose a new password">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {finish.isError ? <Notice tone="error">{describeError(finish.error)}</Notice> : null}
        <TextField label="New password" type="password" autoComplete="new-password" required minLength={12}
          hint="At least 12 characters." value={password} onChange={(e) => setPassword(e.target.value)} />
        <TextField label="Repeat the password" type="password" autoComplete="new-password" required value={confirm}
          onChange={(e) => setConfirm(e.target.value)} error={mismatch ? "The two passwords differ." : undefined} />
        <Button type="submit" variant="primary" busy={finish.isPending} className="justify-center">Change the password</Button>
      </form>
    </AuthFrame>
  );
}
