import { cn } from "@/lib/utils";

import { safeHttpUrl } from "../lib/item-text";

/** An item's title: an external link when the URL is http(s), plain text otherwise. */
export function ItemLink({
  url,
  title,
  className,
}: {
  url: string | undefined;
  title: string;
  className?: string;
}) {
  const href = safeHttpUrl(url);
  if (!href) return <span className={className}>{title}</span>;
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        "rounded-sm text-foreground no-underline outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50",
        className,
      )}
    >
      {title}
    </a>
  );
}
