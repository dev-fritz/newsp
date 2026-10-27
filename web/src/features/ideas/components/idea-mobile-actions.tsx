import { IdeaActions } from "./idea-actions";

type Props = { onEdit: () => void; onDeepen: () => void; deepening: boolean };

/** Sticky action bar above the bottom navigation, below the desktop breakpoint. */
export function IdeaMobileActions(props: Props) {
  return (
    <div className="fixed inset-x-0 bottom-[calc(64px+env(safe-area-inset-bottom))] z-10 grid grid-cols-2 gap-2 border-t border-border bg-card px-4 py-3 lg:hidden">
      <IdeaActions {...props} large />
    </div>
  );
}
