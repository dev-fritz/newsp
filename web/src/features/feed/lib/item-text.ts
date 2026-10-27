const SOURCE_TAGS: Record<string, string> = {
  hackernews: "HN",
  tabnews: "TN",
  reddit: "r/",
  youtube: "YT",
  rss: "RSS",
  lobsters: "LO",
  devto: "DEV",
  mastodon: "M",
  bluesky: "BS",
  github: "GH",
  producthunt: "PH",
};

/** The tiny masthead tag of a source kind. */
export function sourceTag(kind: string | undefined): string {
  if (!kind) return "—";
  return SOURCE_TAGS[kind] ?? kind.slice(0, 3).toUpperCase();
}

/** Only http(s) links from the API are ever rendered as links. */
export function safeHttpUrl(url: string | undefined): string | null {
  if (!url) return null;
  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.href : null;
  } catch {
    return null;
  }
}

const ENTITIES: Record<string, string> = {
  amp: "&",
  lt: "<",
  gt: ">",
  quot: '"',
  apos: "'",
  nbsp: " ",
};

function decodeEntities(text: string): string {
  return text.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (match, body: string) => {
    if (body.startsWith("#")) {
      const code =
        body[1]?.toLowerCase() === "x" ? parseInt(body.slice(2), 16) : Number(body.slice(1));
      return Number.isFinite(code) && code > 0 && code < 0x110000
        ? String.fromCodePoint(code)
        : match;
    }
    return ENTITIES[body.toLowerCase()] ?? match;
  });
}

/**
 * Item summaries may carry Markdown or HTML remnants from the source. The UI shows
 * plain text only (React escapes it), so this just removes the markup noise.
 */
export function toPlainText(raw: string | undefined): string {
  if (!raw) return "";
  return decodeEntities(
    raw
      .replace(/<[^>]*>/g, " ")
      .replace(/!\[([^\]]*)\]\([^)]*\)/g, "$1")
      .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
      .replace(/^\s{0,3}(#{1,6}|>|[-*+])\s+/gm, "")
      .replace(/(\*\*|__|`|~~)/g, "")
      .replace(/(^|\s)[*_]+|[*_]+(?=\s|$)/g, "$1"),
  )
    .replace(/\s+/g, " ")
    .trim();
}
