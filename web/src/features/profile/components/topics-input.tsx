import { X } from "lucide-react";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

const MAX_TOPICS = 20;
const MAX_LENGTH = 60;

type TopicsInputProps = { value: string[]; onChange: (topics: string[]) => void };

/** Chip input: Enter or comma adds, × removes, duplicates are ignored case-insensitively. */
export function TopicsInput({ value, onChange }: TopicsInputProps) {
  const { t } = useTranslation("profile");
  const id = useId();
  const [draft, setDraft] = useState("");

  function commit(...raws: string[]): void {
    let next = value;
    for (const raw of raws) {
      const topic = raw.trim().slice(0, MAX_LENGTH);
      if (!topic || next.length >= MAX_TOPICS) continue;
      if (next.some((v) => v.toLowerCase() === topic.toLowerCase())) continue;
      next = [...next, topic];
    }
    if (next !== value) onChange(next);
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex min-h-[52px] flex-wrap items-center gap-2 rounded-[10px] border border-border-strong bg-card p-2.5 focus-within:ring-[3px] focus-within:ring-ring/50">
        <ul className="contents">
          {value.map((topic) => (
            <li
              key={topic.toLowerCase()}
              className="inline-flex max-w-full items-center gap-1.5 rounded-full bg-muted py-1 pr-1.5 pl-3 text-sm"
            >
              <span className="min-w-0 break-words">{topic}</span>
              <button
                type="button"
                aria-label={t("interests.remove", { topic })}
                className="inline-flex size-6 items-center justify-center rounded-full text-muted-foreground hover:text-foreground pointer-coarse:size-8"
                onClick={() => onChange(value.filter((v) => v !== topic))}
              >
                <X className="size-3.5" aria-hidden />
              </button>
            </li>
          ))}
        </ul>
        <label htmlFor={id} className="flex min-w-40 flex-1">
          <span className="sr-only">{t("interests.label")}</span>
          <input
            id={id}
            type="text"
            value={draft}
            maxLength={MAX_LENGTH}
            disabled={value.length >= MAX_TOPICS}
            placeholder={t("interests.placeholder")}
            className="min-h-7 w-full bg-transparent text-sm outline-none placeholder:text-subtle-foreground"
            onChange={(e) => {
              const next = e.target.value;
              if (next.includes(",")) {
                const parts = next.split(",");
                commit(...parts.slice(0, -1));
                setDraft(parts.at(-1) ?? "");
              } else setDraft(next);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                commit(draft);
                setDraft("");
              } else if (e.key === "Backspace" && !draft && value.length > 0) {
                onChange(value.slice(0, -1));
              }
            }}
            onBlur={() => {
              commit(draft);
              setDraft("");
            }}
          />
        </label>
      </div>
      <p className="text-xs text-subtle-foreground">{t("interests.limit")}</p>
    </div>
  );
}
