import { z } from "zod";

// Server-only configuration. Never import this module from client code.
const schema = z
  .object({
    NODE_ENV: z.enum(["development", "production", "test"]).default("development"),
    // Base URL of the Go API as seen from this server (not from the browser).
    API_URL: z.url().default("http://localhost:8080"),
    // Encrypts the session cookie. At least 32 characters.
    SESSION_SECRET: z.string().min(32, "SESSION_SECRET must have at least 32 characters"),
    // Public origin of the web app; mutations from other origins are refused.
    APP_ORIGIN: z.url().default("http://localhost:5173"),
  })
  .superRefine((env, ctx) => {
    if (env.NODE_ENV !== "production") return;
    if (env.SESSION_SECRET.startsWith("dev-only")) {
      ctx.addIssue({
        code: "custom",
        path: ["SESSION_SECRET"],
        message: "must not use the development default in production",
      });
    }
  });

export type ServerEnv = z.infer<typeof schema>;

let cached: ServerEnv | undefined;

/** Validated server environment, loaded once. */
export function serverEnv(): ServerEnv {
  if (!cached) {
    const parsed = schema.safeParse(process.env);
    if (!parsed.success) {
      const details = parsed.error.issues
        .map((i) => `${i.path.join(".")}: ${i.message}`)
        .join("; ");
      throw new Error(`Invalid web configuration: ${details}`);
    }
    cached = parsed.data;
  }
  return cached;
}
