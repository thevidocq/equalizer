import { useState, type FormEvent } from "react";
import { Navigate, useNavigate } from "react-router";
import { Button, Notice, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useMfaVerify } from "../hooks/useMfa";
import { useLogout, useMe } from "../hooks/useSession";
import { AuthFrame } from "./AuthFrame";

/** Second login step: the code from the app, or a recovery code. */
export function CodePage() {
  const me = useMe();
  const verify = useMfaVerify();
  const logout = useLogout();
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const [recovery, setRecovery] = useState(false);

  if (me.isPending) return null;
  if (!me.data) return <Navigate to="/login" replace />;
  if (me.data.mfa_verified) return <Navigate to="/" replace />;

  function submit(e: FormEvent) {
    e.preventDefault();
    verify.mutate(code, { onSuccess: () => navigate("/", { replace: true }) });
  }

  return (
    <AuthFrame title={recovery ? "Use a recovery code" : "Enter the code from your app"}>
      <form onSubmit={submit} className="flex flex-col gap-4">
        {verify.isError ? <Notice tone="error">{describeError(verify.error)}</Notice> : null}
        <TextField label={recovery ? "Recovery code" : "Six-digit code"} required autoFocus value={code}
          onChange={(e) => setCode(e.target.value)} autoComplete="one-time-code"
          inputMode={recovery ? "text" : "numeric"} placeholder={recovery ? "xxxx-xxxx-xxxx" : "123456"}
          hint={recovery ? "Each recovery code works once." : `The code of Equalizer for ${me.data.email}.`} />
        <Button type="submit" variant="primary" busy={verify.isPending} className="justify-center">Continue</Button>
        <div className="flex justify-between text-sm">
          <Button variant="quiet" onClick={() => { setRecovery(!recovery); setCode(""); }}>
            {recovery ? "Use the app instead" : "I lost my phone"}
          </Button>
          <Button variant="quiet" onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/login") })}>Log out</Button>
        </div>
      </form>
    </AuthFrame>
  );
}
