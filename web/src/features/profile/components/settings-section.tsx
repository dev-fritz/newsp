type SettingsSectionProps = {
  id: string;
  title: string;
  hint: string;
  children: React.ReactNode;
};

/** Two-column settings block: title and hint beside the controls. */
export function SettingsSection({ id, title, hint, children }: SettingsSectionProps) {
  return (
    <section
      aria-labelledby={id}
      className="grid grid-cols-[repeat(auto-fit,minmax(min(260px,100%),1fr))] gap-x-12 gap-y-5 border-t border-border py-8"
    >
      <div className="flex flex-col gap-1.5">
        <h2 id={id} className="text-base font-semibold">
          {title}
        </h2>
        <p className="text-xs text-subtle-foreground">{hint}</p>
      </div>
      <div className="flex min-w-0 flex-col gap-4">{children}</div>
    </section>
  );
}
