/**
 * The start of a report for the home screen: the "what changed" callout plus the
 * first `sections` numbered sections ("## n. heading").
 */
export function reportExcerpt(markdown: string, sections: number): string {
  const text = markdown.trim();
  const headings = [...text.matchAll(/^## /gm)].map((match) => match.index);
  const cut = headings[sections];
  return cut === undefined ? text : text.slice(0, cut).trim();
}
