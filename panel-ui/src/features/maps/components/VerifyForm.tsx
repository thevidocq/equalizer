import { useState } from "react";
import { Button, Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useVerify } from "../hooks/useVerification";
import { TestAccounts, type TestAccount } from "./TestAccounts";

type Props = { app: string; version: number; busy: boolean };

/** Verify a version again, with test accounts for the pages behind their login, and let the agent correct it. */
export function VerifyForm({ app, version, busy }: Props) {
  const verify = useVerify(app, version);
  const [open, setOpen] = useState(false);
  const [accounts, setAccounts] = useState<TestAccount[]>([]);
  const [correct, setCorrect] = useState(false);
  const run = () => verify.mutate({ accounts: accounts.filter((a) => a.username.trim() !== ""), correct },
    { onSuccess: () => { setAccounts([]); setOpen(false); } });
  if (!open) return <div><Button onClick={() => setOpen(true)} disabled={busy}>Verify</Button></div>;
  return (
    <div className="flex w-full flex-col gap-3 border border-rule p-4">
      <TestAccounts value={accounts} onChange={setAccounts} />
      <p className="max-w-prose text-sm text-ink-soft">
        Without test accounts only public pages are verified. The passwords are kept encrypted until the verification ends.
      </p>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={correct} onChange={(e) => setCorrect(e.target.checked)} />
        Let the agent correct this version into a new draft
      </label>
      {verify.isError ? <Notice tone="error">{describeError(verify.error)}</Notice> : null}
      <div className="flex gap-2">
        <Button onClick={run} busy={verify.isPending}>Verify now</Button>
        <Button onClick={() => setOpen(false)}>Cancel</Button>
      </div>
    </div>
  );
}
