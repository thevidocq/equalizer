import type { User } from "@/shared/api/client";

/** Where a logged-in user must go before the panel opens; null when done. */
export function nextStep(u: User | null | undefined): string | null {
  if (!u || u.mfa_verified) return null;
  return u.mfa_enabled ? "/login/code" : "/login/authenticator";
}
