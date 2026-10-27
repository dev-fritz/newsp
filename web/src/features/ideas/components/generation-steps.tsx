import { useTranslation } from "react-i18next";

const STEPS = ["signals", "candidates", "dedup", "save"] as const;

/**
 * The stages a generation goes through; the job does not report which one it is on.
 * Use `as="span"` where only phrasing content is valid (inside a button).
 */
export function GenerationSteps({ as = "ol" }: { as?: "ol" | "span" }) {
  const { t } = useTranslation("ideas");
  const Container = as;
  const Row = as === "ol" ? "li" : "span";
  return (
    <Container
      className={
        as === "ol"
          ? "m-0 flex list-none flex-col gap-3 p-0 text-[13.5px] text-muted-foreground"
          : "m-0 flex flex-col gap-3 p-0 text-[13.5px] text-muted-foreground"
      }
    >
      {STEPS.map((step) => (
        <Row key={step} className="flex items-center gap-2.5">
          <span
            aria-hidden
            className="size-4 shrink-0 rounded-full border-[1.5px] border-border-strong"
          />
          {t(`progress.steps.${step}`)}
        </Row>
      ))}
    </Container>
  );
}
