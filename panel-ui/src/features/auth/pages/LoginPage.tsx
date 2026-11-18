import { useState, type FormEvent } from "react";
import { useLocation, useNavigate } from "react-router";
import { Button, Notice, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useLogin } from "../hooks/useSession";

export function LoginPage() {
  const login = useLogin();
  const navigate = useNavigate();
  const from = (useLocation().state as { from?: string } | null)?.from ?? "/";
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    login.mutate({ email, password }, { onSuccess: () => navigate(from, { replace: true }) });
  }

  return (
    <main className="grid min-h-screen place-items-center px-4">
      <div className="w-full max-w-sm">
        <Wordmark />
        <form onSubmit={submit} className="mt-8 flex flex-col gap-4 rounded-[var(--radius-panel)] border border-rule bg-surface p-6">
          <h1 className="text-lg font-semibold">Log in to the panel</h1>
          {login.isError ? <Notice tone="error">{describeError(login.error)}</Notice> : null}
          <TextField label="Email" type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} />
          <TextField label="Password" type="password" autoComplete="current-password" required value={password}
            onChange={(e) => setPassword(e.target.value)} />
          <Button type="submit" variant="primary" busy={login.isPending} className="justify-center">
            Log in
          </Button>
        </form>
      </div>
    </main>
  );
}

/** The mark: three bars levelled to the same height, what the gateway does to every request. */
export function Wordmark({ inverted = false }: { inverted?: boolean }) {
  const ink = inverted ? "#F4F6F8" : "#1C2633";
  return (
    <div className="flex items-center gap-2.5">
      <svg width="22" height="22" viewBox="0 0 22 22" aria-hidden="true">
        <path d="M5 5v12M11 5v12M17 5v12" stroke={ink} strokeWidth="3" strokeLinecap="round" />
      </svg>
      <span className={`text-[17px] font-bold tracking-[-0.01em] ${inverted ? "text-paper" : "text-ink"}`}>
        Equalizer
      </span>
    </div>
  );
}
