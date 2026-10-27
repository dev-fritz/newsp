// Small word-level diff (longest common subsequence). Pure functions, no
// dependencies, so they can be unit tested on their own.

export type DiffOp = { type: "equal" | "add" | "remove"; text: string };

/** Above this many token pairs the LCS table is skipped and the text is replaced whole. */
const MAX_TABLE_CELLS = 4_000_000;

/** Words, each keeping the whitespace around it, so ops can be joined back losslessly. */
export function tokenize(text: string): string[] {
  return text.match(/\s*\S+\s*/g) ?? (text.length > 0 ? [text] : []);
}

function key(token: string): string {
  return token.trim();
}

function pushOp(ops: DiffOp[], type: DiffOp["type"], text: string): void {
  if (text.length === 0) return;
  const last = ops[ops.length - 1];
  if (last && last.type === type) last.text += text;
  else ops.push({ type, text });
}

/** Word-level diff from `before` to `after`; consecutive ops of a kind are merged. */
export function diffWords(before: string, after: string): DiffOp[] {
  const a = tokenize(before);
  const b = tokenize(after);

  let start = 0;
  while (start < a.length && start < b.length && key(a[start] ?? "") === key(b[start] ?? "")) {
    start++;
  }
  let endA = a.length;
  let endB = b.length;
  while (endA > start && endB > start && key(a[endA - 1] ?? "") === key(b[endB - 1] ?? "")) {
    endA--;
    endB--;
  }

  const ops: DiffOp[] = [];
  pushOp(ops, "equal", b.slice(0, start).join(""));

  const midA = a.slice(start, endA);
  const midB = b.slice(start, endB);
  const n = midA.length;
  const m = midB.length;

  if (n === 0 || m === 0 || n * m > MAX_TABLE_CELLS) {
    pushOp(ops, "remove", midA.join(""));
    pushOp(ops, "add", midB.join(""));
  } else {
    // lcs[i][j] = length of the LCS of midA[i..] and midB[j..].
    const width = m + 1;
    const lcs = new Uint32Array((n + 1) * width);
    for (let i = n - 1; i >= 0; i--) {
      for (let j = m - 1; j >= 0; j--) {
        lcs[i * width + j] =
          key(midA[i] ?? "") === key(midB[j] ?? "")
            ? (lcs[(i + 1) * width + j + 1] ?? 0) + 1
            : Math.max(lcs[(i + 1) * width + j] ?? 0, lcs[i * width + j + 1] ?? 0);
      }
    }
    let i = 0;
    let j = 0;
    while (i < n && j < m) {
      if (key(midA[i] ?? "") === key(midB[j] ?? "")) {
        pushOp(ops, "equal", midB[j] ?? "");
        i++;
        j++;
      } else if ((lcs[(i + 1) * width + j] ?? 0) >= (lcs[i * width + j + 1] ?? 0)) {
        pushOp(ops, "remove", midA[i] ?? "");
        i++;
      } else {
        pushOp(ops, "add", midB[j] ?? "");
        j++;
      }
    }
    pushOp(ops, "remove", midA.slice(i).join(""));
    pushOp(ops, "add", midB.slice(j).join(""));
  }

  pushOp(ops, "equal", b.slice(endB).join(""));
  return ops;
}

/** True when the ops contain any addition or removal. */
export function hasChanges(ops: readonly DiffOp[]): boolean {
  return ops.some((op) => op.type !== "equal");
}

export type ListOp<T> = { type: "equal" | "add" | "remove"; item: T };

/** Item-by-item diff of two lists (LCS over `keyOf`); equal items come from `after`. */
export function diffItems<T>(
  before: readonly T[],
  after: readonly T[],
  keyOf: (item: T) => string,
): ListOp<T>[] {
  const n = before.length;
  const m = after.length;
  const width = m + 1;
  const lcs = new Uint32Array((n + 1) * width);
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      const x = before[i];
      const y = after[j];
      lcs[i * width + j] =
        x !== undefined && y !== undefined && keyOf(x) === keyOf(y)
          ? (lcs[(i + 1) * width + j + 1] ?? 0) + 1
          : Math.max(lcs[(i + 1) * width + j] ?? 0, lcs[i * width + j + 1] ?? 0);
    }
  }
  const out: ListOp<T>[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    const x = before[i];
    const y = after[j];
    if (x === undefined || y === undefined) break;
    if (keyOf(x) === keyOf(y)) {
      out.push({ type: "equal", item: y });
      i++;
      j++;
    } else if ((lcs[(i + 1) * width + j] ?? 0) >= (lcs[i * width + j + 1] ?? 0)) {
      out.push({ type: "remove", item: x });
      i++;
    } else {
      out.push({ type: "add", item: y });
      j++;
    }
  }
  for (; i < n; i++) {
    const x = before[i];
    if (x !== undefined) out.push({ type: "remove", item: x });
  }
  for (; j < m; j++) {
    const y = after[j];
    if (y !== undefined) out.push({ type: "add", item: y });
  }
  return out;
}

export type PairedOp<T> =
  | { type: "equal"; item: T }
  | { type: "add"; item: T }
  | { type: "remove"; item: T }
  | { type: "change"; before: T; after: T };

/**
 * Like diffItems, but a removed item and an added item that sit at the same place
 * are reported as one "change" so the caller can show what changed inside it.
 */
export function diffPaired<T>(
  before: readonly T[],
  after: readonly T[],
  keyOf: (item: T) => string,
): PairedOp<T>[] {
  const ops = diffItems(before, after, keyOf);
  const out: PairedOp<T>[] = [];
  let index = 0;
  while (index < ops.length) {
    const op = ops[index];
    if (!op) break;
    if (op.type === "equal") {
      out.push(op);
      index++;
      continue;
    }
    const removed: T[] = [];
    const added: T[] = [];
    while (index < ops.length) {
      const run = ops[index];
      if (!run || run.type === "equal") break;
      (run.type === "remove" ? removed : added).push(run.item);
      index++;
    }
    const pairs = Math.min(removed.length, added.length);
    for (let k = 0; k < pairs; k++) {
      const b = removed[k];
      const a = added[k];
      if (b !== undefined && a !== undefined) out.push({ type: "change", before: b, after: a });
    }
    for (const item of removed.slice(pairs)) out.push({ type: "remove", item });
    for (const item of added.slice(pairs)) out.push({ type: "add", item });
  }
  return out;
}
