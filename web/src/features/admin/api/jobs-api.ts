import { getData } from "@/lib/http";

import type { Job } from "../types";

export function getJob(id: string): Promise<Job> {
  return getData<Job>(`/jobs/${encodeURIComponent(id)}`);
}
