type PageHeaderProps = {
  kicker?: string;
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
};

/** Editorial page title: kicker, serif headline and optional actions. */
export function PageHeader({ kicker, title, description, actions }: PageHeaderProps) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-5">
      <div className="flex min-w-0 flex-[1_1_420px] flex-col gap-2.5">
        {kicker && <span className="kicker">{kicker}</span>}
        <h1 className="font-serif text-4xl leading-[1.05] font-medium text-balance sm:text-[42px]">
          {title}
        </h1>
        {description && (
          <p className="max-w-2xl text-[15px] text-muted-foreground">{description}</p>
        )}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </header>
  );
}
