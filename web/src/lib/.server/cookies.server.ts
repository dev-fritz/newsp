/** Reads one cookie from the request. */
export function readCookie(request: Request, name: string): string | null {
  const header = request.headers.get("Cookie");
  if (!header) return null;
  for (const part of header.split(";")) {
    const [k, ...rest] = part.trim().split("=");
    if (k === name) return decodeURIComponent(rest.join("="));
  }
  return null;
}

/** Preference cookie readable by the page (theme, language); never identity. */
export function preferenceCookie(name: string, value: string): string {
  return `${name}=${encodeURIComponent(value)}; Path=/; SameSite=Lax; Max-Age=31536000`;
}
