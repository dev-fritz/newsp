import { createCipheriv, createDecipheriv, createHash, randomBytes } from "node:crypto";

import { serverEnv } from "@/config/env.server";
import { readCookie } from "@/lib/.server/cookies.server";

/** The signed-in user, as the API describes it. */
export type SessionUser = {
  id: string;
  name: string;
  email: string;
  role: "admin" | "member";
  locale: "pt-BR" | "en";
  permissions: string[];
  /** Cookies sealed before this field existed lack it: read those as verified. */
  emailVerified: boolean;
};

/** Everything the BFF keeps for a signed-in browser. Never sent to client JS. */
export type SessionData = {
  accessToken: string;
  accessExpiresAt: string;
  refreshToken: string;
  refreshExpiresAt: string;
  user: SessionUser;
};

export const SESSION_COOKIE = "newsp_session";

function key(): Buffer {
  // Derive a fixed-length AES key from the configured secret.
  return createHash("sha256").update(serverEnv().SESSION_SECRET).digest();
}

/** AES-256-GCM: confidentiality and integrity for the cookie payload. */
export function sealSession(data: SessionData): string {
  const iv = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", key(), iv);
  const body = Buffer.concat([cipher.update(JSON.stringify(data), "utf8"), cipher.final()]);
  return Buffer.concat([iv, cipher.getAuthTag(), body]).toString("base64url");
}

/** Returns the session, or null when the cookie is missing, tampered or stale. */
export function openSession(sealed: string): SessionData | null {
  try {
    const raw = Buffer.from(sealed, "base64url");
    const decipher = createDecipheriv("aes-256-gcm", key(), raw.subarray(0, 12));
    decipher.setAuthTag(raw.subarray(12, 28));
    const plain = Buffer.concat([decipher.update(raw.subarray(28)), decipher.final()]).toString(
      "utf8",
    );
    const data = JSON.parse(plain) as SessionData;
    if (!data.refreshToken || new Date(data.refreshExpiresAt).getTime() <= Date.now()) return null;
    return data;
  } catch {
    return null;
  }
}

export function getSession(request: Request): SessionData | null {
  const value = readCookie(request, SESSION_COOKIE);
  return value ? openSession(value) : null;
}

function cookieAttributes(maxAge: number): string {
  const secure = serverEnv().NODE_ENV === "production" ? "; Secure" : "";
  return `Path=/; HttpOnly; SameSite=Lax; Max-Age=${maxAge}${secure}`;
}

/** Set-Cookie header storing the session until its refresh token expires. */
export function commitSession(data: SessionData): string {
  const maxAge = Math.max(
    0,
    Math.floor((new Date(data.refreshExpiresAt).getTime() - Date.now()) / 1000),
  );
  return `${SESSION_COOKIE}=${sealSession(data)}; ${cookieAttributes(maxAge)}`;
}

/** Set-Cookie header that removes the session. */
export function destroySession(): string {
  return `${SESSION_COOKIE}=; ${cookieAttributes(0)}`;
}
