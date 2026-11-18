import { useState } from "react";
import { Button, Notice } from "@/shared/ui";

/** Shown once: each code replaces the app for one login. */
export function RecoveryCodes({ codes, onDone }: { codes: string[]; onDone: () => void }) {
  const [saved, setSaved] = useState(false);
  const text = codes.join("\n");
  return (
    <div className="flex flex-col gap-4">
      <Notice tone="info">
        If you lose your phone, each of these codes lets you log in once. They are shown only now: save them in your password
        manager or print them.
      </Notice>
      <ol className="grid grid-cols-2 gap-x-6 gap-y-1 rounded-[var(--radius-control)] bg-paper p-4 font-mono text-sm">
        {codes.map((c) => <li key={c}>{c}</li>)}
      </ol>
      <div className="flex flex-wrap gap-2">
        <Button onClick={() => navigator.clipboard.writeText(text)}>Copy codes</Button>
        <Button onClick={() => {
          const a = document.createElement("a");
          a.href = URL.createObjectURL(new Blob([text + "\n"], { type: "text/plain" }));
          a.download = "equalizer-recovery-codes.txt";
          a.click();
        }}>Download</Button>
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} />
        I saved the recovery codes
      </label>
      <Button variant="primary" disabled={!saved} onClick={onDone} className="justify-center">Open the panel</Button>
    </div>
  );
}
