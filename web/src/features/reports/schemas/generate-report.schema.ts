import { z } from "zod";

export const generateReportSchema = z.object({
  window_hours: z.enum(["6", "24", "168", "720"]),
  focus: z.string().trim().max(200, "max"),
});

export type GenerateReportValues = z.infer<typeof generateReportSchema>;
