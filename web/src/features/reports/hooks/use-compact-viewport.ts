import { useSyncExternalStore } from "react";

const QUERY = "(pointer: coarse), (max-width: 767px)";

function subscribe(onChange: () => void): () => void {
  const media = window.matchMedia(QUERY);
  media.addEventListener("change", onChange);
  return () => media.removeEventListener("change", onChange);
}

/** True on touch devices and narrow viewports, where popovers become bottom sheets. */
export function useCompactViewport(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => window.matchMedia(QUERY).matches,
    () => false,
  );
}
