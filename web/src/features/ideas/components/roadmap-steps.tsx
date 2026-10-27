export type RoadmapStep = { key: string; title: string; detail?: string; period?: string };

type RoadmapStepsProps = { steps: RoadmapStep[]; "aria-labelledby"?: string };

/** Steps as a vertical roadmap: numbered nodes joined by a line, with an optional period above each title. */
export function RoadmapSteps({ steps, ...aria }: RoadmapStepsProps) {
  return (
    <ol {...aria} className="m-0 flex list-none flex-col p-0">
      {steps.map((step, index) => {
        const last = index === steps.length - 1;
        return (
          <li key={step.key} className="grid grid-cols-[28px_minmax(0,1fr)] gap-x-4">
            <div className="flex flex-col items-center" aria-hidden>
              <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary font-mono text-xs font-semibold text-primary-foreground">
                {index + 1}
              </span>
              {!last && <span className="my-1 w-px flex-1 bg-border-strong" />}
            </div>
            <div className={last ? "min-w-0 pt-[3px]" : "min-w-0 pt-[3px] pb-5"}>
              {step.period && (
                <span className="block font-mono text-xs text-subtle-foreground uppercase">
                  {step.period}
                </span>
              )}
              <p className="m-0 text-[15px] font-semibold break-words">{step.title}</p>
              {step.detail && (
                <p className="m-0 mt-1 text-sm break-words text-muted-foreground">{step.detail}</p>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
