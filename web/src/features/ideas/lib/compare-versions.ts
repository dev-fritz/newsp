/**
 * Picks the pair of versions to compare. Requested numbers that do not exist are
 * ignored; the default is the previous version against the target.
 */
export function resolveCompareVersions(
  available: readonly number[],
  current: number,
  from?: number,
  to?: number,
): { from: number; to: number } {
  const sorted = available.toSorted((a, b) => a - b);
  const target = to !== undefined && sorted.includes(to) ? to : current;
  const earlier = sorted.filter((v) => v < target);
  const base =
    from !== undefined && sorted.includes(from) ? from : (earlier[earlier.length - 1] ?? target);
  return { from: base, to: target };
}
