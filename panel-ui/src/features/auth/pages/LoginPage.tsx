import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router";
import { Button, Notice, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useLogin } from "../hooks/useSession";
import { AuthFrame } from "./AuthFrame";
import { nextStep } from "./steps";

export function LoginPage() {
  const login = useLogin();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    login.mutate({ email, password }, { onSuccess: (u) => navigate(nextStep(u) ?? "/", { replace: true }) });
  }

  return (
    <AuthFrame title="Log in to the panel">
      <form onSubmit={submit} className="flex flex-col gap-4">
        {login.isError ? <Notice tone="error">{describeError(login.error)}</Notice> : null}
        <TextField label="Email" type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} />
        <TextField label="Password" type="password" autoComplete="current-password" required value={password}
          onChange={(e) => setPassword(e.target.value)} />
        <Button type="submit" variant="primary" busy={login.isPending} className="justify-center">Log in</Button>
      </form>
      <Link to="/forgot" className="text-sm text-signal hover:underline">Forgot your password?</Link>
    </AuthFrame>
  );
}
