import type { Schemas } from "./client";

type Problem = NonNullable<Schemas["Invitation"]["send_problem"]>;

const reasons: Record<Problem, string> = {
  not_configured: "email sending is not configured on this installation",
  unreachable: "the mail server could not be reached",
  tls: "the mail server refused the secure connection",
  auth_rejected: "the mail server refused the account or password",
  sender_rejected: "the mail server refused the sender address (EQ_MAIL_FROM)",
  recipient_rejected: "the mail server refused the recipient address",
  rejected: "the mail server refused the message",
};

/** Why an invitation email was not sent, in a sentence for a toast. */
export function notSentReason(problem: Problem | undefined): string {
  return problem ? reasons[problem] : "the mail server did not accept it";
}
