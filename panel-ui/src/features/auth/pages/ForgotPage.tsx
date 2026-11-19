import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { Button, Notice, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useRequestReset } from "../hooks/useReset";
import { AuthFrame } from "./AuthFrame";

export function ForgotPage() {
  const request = useRequestReset();
  const [email, setEmail] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    request.mutate(email);
  }

  if (request.isSuccess) {
    return (
      <AuthFrame title="Check your email">
        <p className="text-sm text-ink-soft">
          If an account exists for <strong className="text-ink">{email}</strong>, an email with a link to choose a new
          password is on its way. The link works once, for one hour.
        </p>
        <Link to="/login" className="text-sm font-semibold text-signal hover:underline">Back to the login</Link>
      </AuthFrame>
    );
  }
  return (
    <AuthFrame title="Forgotten password">
      <p className="text-sm text-ink-soft">Enter the email of your account: you will receive a link to choose a new password.</p>
      <form onSubmit={submit} className="flex flex-col gap-4">
        {request.isError ? <Notice tone="error">{describeError(request.error)}</Notice> : null}
        <TextField label="Email" type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} />
        <Button type="submit" variant="primary" busy={request.isPending} className="justify-center">Send the link</Button>
      </form>
      <Link to="/login" className="text-sm text-signal hover:underline">Back to the login</Link>
    </AuthFrame>
  );
}
