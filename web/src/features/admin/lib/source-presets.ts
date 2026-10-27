import type { SourceKind } from "../types";

export type SourcePreset = {
  id: string;
  kind: SourceKind;
  name: string;
  config: Record<string, unknown>;
  intervalMinutes: number;
};

/** Suggested first sources of the setup wizard. */
export const SOURCE_PRESETS: SourcePreset[] = [
  {
    id: "hackernews-top",
    kind: "hackernews",
    name: "Hacker News",
    config: { list: "top", limit: 30 },
    intervalMinutes: 15,
  },
  {
    id: "tabnews-relevant",
    kind: "tabnews",
    name: "TabNews",
    config: { strategy: "relevant", limit: 30 },
    intervalMinutes: 30,
  },
  {
    id: "bbc-technology",
    kind: "rss",
    name: "BBC · Technology",
    config: { url: "https://feeds.bbci.co.uk/news/technology/rss.xml", limit: 30 },
    intervalMinutes: 60,
  },
];
