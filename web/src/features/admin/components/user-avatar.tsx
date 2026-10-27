import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  const first = parts[0]?.[0] ?? "?";
  const last = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? "") : "";
  return (first + last).toUpperCase();
}

/** Round avatar with the person's initials; admins get the inverted look. */
export function UserAvatar({
  name,
  inverted,
  muted,
}: {
  name: string;
  inverted?: boolean;
  muted?: boolean;
}) {
  return (
    <Avatar className="size-[34px]" aria-hidden>
      <AvatarFallback
        className={cn(
          "text-xs font-semibold",
          inverted ? "bg-foreground text-background" : "bg-muted text-foreground",
          muted && "text-subtle-foreground",
        )}
      >
        {initials(name)}
      </AvatarFallback>
    </Avatar>
  );
}
