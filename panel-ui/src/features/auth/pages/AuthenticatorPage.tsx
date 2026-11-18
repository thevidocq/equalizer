import { useEffect, useRef, useState, type FormEvent } from "react";
import { Navigate, useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { Button, Notice, TextField } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useMfaEnable, useMfaSetup } from "../hooks/useMfa";
import { useMe } from "../hooks/useSession";
import { QrCode } from "../components/QrCode";
import { RecoveryCodes } from "../components/RecoveryCodes";
import { AuthFrame } from "./AuthFrame";

/** First login: link an authenticator app, then keep the recovery codes. */
export function AuthenticatorPage() {
  const me = useMe();
  const setup = useMfaSetup();
  const enable = useMfaEnable();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const needsSetup = !!me.data && !me.data.mfa_enabled;
  const started = useRef(false);

  useEffect(() => {
    // Once only: a second setup would replace the secret shown in the QR code.
    if (needsSetup && !started.current) {
      started.current = true;
      setup.mutate();
    }
  }, [needsSetup, setup]);

  if (me.isPending) return null;
  if (!me.data) return <Navigate to="/login" replace />;

  if (enable.data) {
    return (
      <AuthFrame title="Keep your recovery codes" wide>
        <RecoveryCodes codes={enable.data.recovery_codes} onDone={async () => {
          await qc.invalidateQueries({ queryKey: ["me"] });
          navigate("/", { replace: true });
        }} />
      </AuthFrame>
    );
  }
  if (!needsSetup) return <Navigate to="/" replace />;

  function submit(e: FormEvent) {
    e.preventDefault();
    enable.mutate(code);
  }

  return (
    <AuthFrame title="Set up your authenticator app" wide>
      <p className="text-sm text-ink-soft">
        The panel asks for a code from an app on your phone at every login (Google Authenticator, Microsoft Authenticator,
        1Password, Aegis…). Scan the code with the app.
      </p>
      {setup.isError ? <Notice tone="error">{describeError(setup.error)}</Notice> : null}
      {setup.data ? (
        <div className="flex flex-col items-start gap-3">
          <QrCode value={setup.data.otpauth_uri} label="QR code for the authenticator app" />
          <p className="text-sm text-ink-soft">
            Cannot scan? Type this key in the app:{" "}
            <span className="break-all font-mono text-ink">{setup.data.secret.replace(/(.{4})/g, "$1 ").trim()}</span>
          </p>
        </div>
      ) : null}
      <form onSubmit={submit} className="flex flex-col gap-4">
        {enable.isError ? <Notice tone="error">{describeError(enable.error)}</Notice> : null}
        <TextField label="Six-digit code from the app" required inputMode="numeric" autoComplete="one-time-code"
          value={code} onChange={(e) => setCode(e.target.value)} />
        <Button type="submit" variant="primary" busy={enable.isPending} disabled={!setup.data} className="justify-center">
          Turn on
        </Button>
      </form>
    </AuthFrame>
  );
}
