/**
 * Returns `to` when it is an internal path, otherwise `fallback`. Blocks open
 * redirects such as "//evil.com", "/\\evil.com" or "https://evil.com".
 */
export function safeRedirect(
  to: FormDataEntryValue | string | null | undefined,
  fallback = "/",
): string {
  if (
    typeof to !== "string" ||
    !to.startsWith("/") ||
    to.startsWith("//") ||
    to.startsWith("/\\")
  ) {
    return fallback;
  }
  try {
    // Resolving against a dummy origin catches encoded tricks like "/%5C%5Cevil.com".
    const url = new URL(to, "http://internal.invalid");
    if (url.origin !== "http://internal.invalid") return fallback;
    return url.pathname + url.search + url.hash;
  } catch {
    return fallback;
  }
}
