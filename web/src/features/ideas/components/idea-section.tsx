import { useId } from "react";

type IdeaSectionProps = { title: string; children: React.ReactNode; id?: string };

/** An article section: serif heading and its content. */
export function IdeaSection({ title, children, id }: IdeaSectionProps) {
  const headingId = useId();
  return (
    <section id={id} aria-labelledby={headingId} className="flex scroll-mt-24 flex-col gap-3.5">
      <h2
        id={headingId}
        className="font-serif text-[26px] leading-tight font-semibold sm:text-[28px]"
      >
        {title}
      </h2>
      {children}
    </section>
  );
}
