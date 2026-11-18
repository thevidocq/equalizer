import { ApiError } from "./client";

const messages: Record<string, string> = {
  invalid_credentials: "Email or password is wrong.",
  too_many_attempts: "Too many failed attempts. Wait 15 minutes and try again.",
  already_exists: "Something with this id already exists.",
  forbidden: "Your role does not allow this action.",
  author_cannot_decide: "Someone other than the author must approve or reject this version.",
  wrong_state: "The version changed in the meantime. Reload the page.",
  invalid_map: "The map does not follow the format.",
  weak_password: "The password needs at least 12 characters.",
  not_found: "Not found.",
  unauthorized: "Your session ended. Log in again.",
};

/** A sentence that tells the user what happened and what to do. */
export function describeError(err: unknown): string {
  if (err instanceof ApiError) return messages[err.code] ?? err.message ?? "The panel could not complete the request.";
  if (err instanceof Error) return err.message;
  return "The panel could not complete the request.";
}
