type SettingsSectionProps = {
  title: string;
  description: string;
  children: React.ReactNode;
};

/** A row of the instance settings: explanation on the left, controls on the right. */
export function SettingsSection({ title, description, children }: SettingsSectionProps) {
  return (
    <div className="grid gap-4 border-t border-border py-6 md:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)] md:gap-10">
      <div className="flex flex-col gap-1.5">
        <h3 className="text-[15px] font-semibold">{title}</h3>
        <p className="text-xs text-subtle-foreground">{description}</p>
      </div>
      <div className="flex flex-col gap-3">{children}</div>
    </div>
  );
}
