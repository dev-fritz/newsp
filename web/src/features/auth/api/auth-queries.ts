import { useMutation } from "@tanstack/react-query";

import type { ApiError } from "@/lib/api-error";

import { resendVerificationEmail } from "./auth-api";

export function useResendVerificationEmail() {
  return useMutation<void, ApiError>({ mutationFn: resendVerificationEmail });
}
