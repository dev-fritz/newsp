import type { components } from "@/types/api";

type Schemas = components["schemas"];

export type FeedItem = Schemas["dtos.ItemResponse"];
export type FeedSource = Schemas["dtos.SourceSummary"];
