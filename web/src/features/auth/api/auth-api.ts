import { send } from "@/lib/http";

/** Asks the API to mail a fresh confirmation link to the signed-in user. */
export function resendVerificationEmail(): Promise<void> {
  return send<void>("post", "/me/verify-email/resend");
}
