import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import type { DiffField, DiffRow, RowStatus, SectionId } from "../lib/idea-diff";
import { DiffText } from "./diff-text";

const MARKER: Record<RowStatus, string> = { equal: "", change: "~", add: "+", remove: "−" };

function RowMarker({ status }: { status: RowStatus }) {
  const { t } = useTranslation("ideas");
  return (
    <span
      className={cn(
        "w-4 shrink-0 text-center font-mono text-xs font-semibold",
        status === "add" && "text-positive",
        status === "remove" && "text-negative",
        status === "change" && "text-warning",
      )}
    >
      <span aria-hidden>{MARKER[status]}</span>
      {status !== "equal" && <span className="sr-only">{t(`compare.row.${status}`)}</span>}
    </span>
  );
}

function field(row: DiffRow, name: string): DiffField {
  return row.fields.find((f) => f.name === name) ?? { name, ops: [] };
}

function Mapped({ ops }: { ops: DiffField["ops"] }) {
  const { t } = useTranslation("ideas");
  return (
    <DiffText
      ops={ops.map((op) => ({ ...op, text: t(`audience.${op.text}`, { defaultValue: op.text }) }))}
    />
  );
}

function RowBody({ sectionId, row }: { sectionId: SectionId; row: DiffRow }) {
  const dim = row.status === "equal" ? "text-muted-foreground" : "";
  switch (sectionId) {
    case "audience":
      return (
        <div className={cn("flex min-w-0 flex-col gap-0.5", dim)}>
          <span className="text-xs text-subtle-foreground">
            <Mapped ops={field(row, "kind").ops} />
          </span>
          <span className="font-semibold">
            <DiffText ops={field(row, "label").ops} />
          </span>
          <span className="text-sm">
            <DiffText ops={field(row, "description").ops} />
          </span>
        </div>
      );
    case "mvp_plan":
      return (
        <div
          className={cn(
            "flex min-w-0 flex-col gap-1 sm:grid sm:grid-cols-[120px_minmax(0,1fr)] sm:gap-4",
            dim,
          )}
        >
          <span className="font-mono text-xs text-subtle-foreground uppercase sm:pt-[3px]">
            <DiffText ops={field(row, "period").ops} />
          </span>
          <span className="text-[15px]">
            <strong className="font-semibold">
              <DiffText ops={field(row, "title").ops} />
            </strong>{" "}
            <DiffText ops={field(row, "description").ops} />
          </span>
        </div>
      );
    case "competitors":
      return (
        <div className={cn("flex min-w-0 flex-col gap-0.5", dim)}>
          <span className="font-semibold">
            <DiffText ops={field(row, "name").ops} />
          </span>
          <span className="text-sm">
            <DiffText ops={field(row, "strengths").ops} />
          </span>
          <span className="text-sm">
            <DiffText ops={field(row, "gap").ops} />
          </span>
        </div>
      );
    default:
      return (
        <span className={cn("min-w-0 text-[15px] break-words", dim)}>
          <DiffText ops={field(row, "value").ops} />
        </span>
      );
  }
}

type Props = { sectionId: SectionId; rows: readonly DiffRow[]; onlyChanges: boolean };

/** A list-like section compared item by item, each row with its +, − or ~ marker. */
export function DiffRows({ sectionId, rows, onlyChanges }: Props) {
  const visible = onlyChanges ? rows.filter((row) => row.status !== "equal") : rows;
  return (
    <ul className="m-0 flex list-none flex-col gap-2 p-0">
      {visible.map((row, position) => (
        <li
          key={`${position}-${row.status}`}
          className={cn(
            "flex items-baseline gap-3 rounded-lg px-3 py-2.5",
            row.status === "add" && "bg-positive-soft/60",
            row.status === "remove" && "bg-negative-soft/60",
            row.status === "change" && "bg-muted/60",
          )}
        >
          <RowMarker status={row.status} />
          <RowBody sectionId={sectionId} row={row} />
        </li>
      ))}
    </ul>
  );
}
