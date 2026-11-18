import * as RT from "@radix-ui/react-toast";
import { createContext, useCallback, useContext, useState, type ReactNode } from "react";

type Toast = { id: number; text: string };
const Ctx = createContext<(text: string) => void>(() => {});

/** Confirms an action in the words of its button ("Published"). */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const show = useCallback((text: string) => setToasts((t) => [...t, { id: Date.now() + Math.random(), text }]), []);
  return (
    <Ctx.Provider value={show}>
      <RT.Provider duration={4000}>
        {children}
        {toasts.map((t) => (
          <RT.Root
            key={t.id}
            onOpenChange={(open) => !open && setToasts((all) => all.filter((x) => x.id !== t.id))}
            className="rounded-[var(--radius-control)] bg-ink px-4 py-3 text-sm text-paper"
          >
            <RT.Description>{t.text}</RT.Description>
          </RT.Root>
        ))}
        <RT.Viewport className="fixed bottom-4 right-4 z-50 flex w-80 flex-col gap-2" />
      </RT.Provider>
    </Ctx.Provider>
  );
}

export function useToast() {
  return useContext(Ctx);
}
