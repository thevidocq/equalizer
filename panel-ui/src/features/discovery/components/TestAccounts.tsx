import { Button, TextField } from "@/shared/ui";

export type TestAccount = { label: string; username: string; password: string };

const MAX = 3;

type Props = { value: TestAccount[]; onChange: (v: TestAccount[]) => void };

/** Up to three test accounts, each explored in its own pass. */
export function TestAccounts({ value, onChange }: Props) {
  const rows = value.length ? value : [{ label: "", username: "", password: "" }];
  const set = (n: number, patch: Partial<TestAccount>) => onChange(rows.map((r, i) => (i === n ? { ...r, ...patch } : r)));
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="text-sm font-semibold">Test accounts (optional)</legend>
      <p className="max-w-prose text-sm text-ink-soft">
        Each account is explored in its own pass, to reach the pages behind its login (for example a customer and an
        administrator). Give accounts with the fewest permissions that still see those pages. Passwords are kept encrypted
        and deleted at the end.
      </p>
      {rows.map((r, n) => (
        <div key={n} className="grid gap-4 sm:grid-cols-3">
          <TextField label="Name in the report" placeholder={`account ${n + 1}`} value={r.label} maxLength={60}
            onChange={(e) => set(n, { label: e.target.value })} />
          <TextField label="Username" autoComplete="off" value={r.username} onChange={(e) => set(n, { username: e.target.value })} />
          <TextField label="Password" type="password" autoComplete="new-password" value={r.password}
            onChange={(e) => set(n, { password: e.target.value })} />
        </div>
      ))}
      {rows.length < MAX ? (
        <div>
          <Button type="button" onClick={() => onChange([...rows, { label: "", username: "", password: "" }])}>Add an account</Button>
        </div>
      ) : null}
    </fieldset>
  );
}
