/** Removes the Markdown that would look odd in a one-line teaser. */
function plain(markdown: string): string {
  return markdown
    .replace(/\s*\[(E\d+(?:\s*,\s*E\d+)*)\]/g, "")
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/[*_`>#]/g, "")
    .replace(/^\s*[-+]\s+/gm, "")
    .replace(/\s+/g, " ")
    .trim();
}

/** The first sentence of a Markdown text, for the "why / for whom / how" strip. */
export function firstSentence(markdown: string): string {
  const text = plain(markdown);
  const match = text.match(/^.*?[.!?](?=\s|$)/);
  return match ? match[0] : text;
}

/**
 * Splits an architecture step written as "Component: what it does" into its
 * name and description. A step without a short name before ": " is all title.
 */
export function splitStep(step: string): { title: string; detail: string } {
  const match = step.trim().match(/^([^:\n]{1,80}):\s+([\s\S]+)$/);
  if (!match?.[1] || !match[2]) return { title: step.trim(), detail: "" };
  return { title: match[1].trim(), detail: match[2].trim() };
}

export const CITE_HASH_PREFIX = "#cite-";

/**
 * Turns "[E1]" and "[E1, E2]" into Markdown links to "#cite-E1", which the
 * renderer draws as chips that jump to the evidence entry.
 */
export function linkCitations(markdown: string): string {
  return markdown.replace(/\[(E\d+(?:\s*,\s*E\d+)*)\](?!\()/g, (_all, refs: string) =>
    refs
      .split(",")
      .map((ref) => ref.trim())
      .map((ref) => `[${ref}](${CITE_HASH_PREFIX}${ref})`)
      .join(""),
  );
}

export function evidenceDomId(ref: string): string {
  return `evidence-${ref}`;
}

/** Two or three letters that tag where an evidence item came from. */
export function sourceTag(kind: string, name: string): string {
  switch (kind) {
    case "hackernews":
      return "HN";
    case "tabnews":
      return "TN";
    case "reddit":
      return "r/";
    case "youtube":
      return "YT";
    case "rss":
      return "RSS";
    case "lobsters":
      return "LO";
    case "devto":
      return "DEV";
    case "mastodon":
      return "M";
    case "bluesky":
      return "BS";
    case "github":
      return "GH";
    case "producthunt":
      return "PH";
    default:
      return (name || kind || "?").slice(0, 2).toUpperCase();
  }
}

/** Only web links from the data are made clickable. */
export function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}

/** Maps an API field path ("content.audience[0].label") onto the edit form's field names. */
export function formFieldPath(apiPath: string): string {
  return apiPath
    .replace(/^content\./, "")
    .replace(/\[(\d+)\]/g, ".$1")
    .replace(/^(stack|architecture_steps)\.\d+$/, "$1");
}
