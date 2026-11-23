import { ApiError } from "./client";

const messages: Record<string, string> = {
  invalid_credentials: "Email or password is wrong.",
  too_many_attempts: "Too many failed attempts. Wait 15 minutes and try again.",
  already_exists: "Something with this id already exists.",
  forbidden: "Your role does not allow this action.",
  wrong_state: "The version changed in the meantime. Reload the page.",
  invalid_map: "The map does not follow the format.",
  weak_password: "The password needs at least 12 characters.",
  not_found: "Not found.",
  unauthorized: "Your session ended. Log in again.",
  invalid_code: "The code is wrong or expired. Use the code shown now in the app.",
  mfa_required: "Complete the second login step first.",
  wrong_password: "The current password is wrong.",
  cannot_deactivate_self: "You cannot deactivate yourself: ask another administrator.",
};

/** Said when the server gave a code but no words (a proxy may drop the body). */
const fallbacks: Record<string, string> = {
  ai_failed: "The AI provider refused the request: check the key and the model in Settings → AI.",
  no_ai: "No AI model is set: an administrator sets one in Settings → AI.",
};

/** A sentence that tells the user what happened and what to do. */
export function describeError(err: unknown): string {
  if (err instanceof ApiError) {
    return messages[err.code] || err.message || fallbacks[err.code] || `The panel could not complete the request (${err.status}).`;
  }
  if (err instanceof Error && err.message) return err.message;
  return "The panel could not complete the request.";
}
