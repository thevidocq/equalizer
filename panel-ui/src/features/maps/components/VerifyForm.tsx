import { useState } from "react";
import { Button, Notice } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useVerify } from "../hooks/useVerification";
import { TestAccounts, type TestAccount } from "./TestAccounts";

type Props = { app: string; version: number; checked: boolean };

/**
 * Check this version on the site again: optional test accounts for the
 * pages behind their login, and whether the agent may prepare a corrected
 * draft. It changes nothing by itself. Every discovery's draft is checked
 * already, so on a checked version this is only a quiet "Check again"; a
 * version never checked (a draft changed by hand) gets the button.
 */
export function VerifyForm({ app, version, checked }: Props) {
  const verify = useVerify(app, version);
  const [open, setOpen] = useState(false);
  const [accounts, setAccounts] = useState<TestAccount[]>([]);
  const [correct, setCorrect] = useState(false);
  const run = () => verify.mutate({ accounts: accounts.filter((a) => a.username.trim() !== ""), correct },
    { onSuccess: () => { setAccounts([]); setOpen(false); } });
  if (!open) {
    return (
      <div>
        {checked
          ? <Button variant="quiet" onClick={() => setOpen(true)}>Check again</Button>
          : <Button variant="primary" onClick={() => setOpen(true)}>Check this version on the site</Button>}
      </div>
    );
  }
  return (
    <div className="flex w-full flex-col gap-3 border border-rule p-4">
      <TestAccounts value={accounts} onChange={setAccounts} />
      <p className="max-w-prose text-sm text-ink-soft">
        Without test accounts only the public pages are checked. The passwords are kept encrypted until the check ends.
      </p>
      <label className="flex items-start gap-2 text-sm">
        <input type="checkbox" className="mt-1" checked={correct} onChange={(e) => setCorrect(e.target.checked)} />
        <span>
          If a page breaks or an attack gets through, let the agent prepare a corrected version as a new draft.
          <span className="block text-ink-soft">This version stays as it is; you decide whether to publish the new one.</span>
        </span>
      </label>
      {verify.isError ? <Notice tone="error">{describeError(verify.error)}</Notice> : null}
      <div className="flex gap-2">
        <Button variant="primary" onClick={run} busy={verify.isPending}>Start the check</Button>
        <Button onClick={() => setOpen(false)}>Cancel</Button>
      </div>
    </div>
  );
}
