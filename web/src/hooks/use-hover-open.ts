import { useEffect, useRef, useState } from "react";

const OPEN_DELAY_MS = 150;
const CLOSE_DELAY_MS = 200;

const isMouse = (event: React.PointerEvent) => event.pointerType === "mouse";

type HoverOpen = {
  open: boolean;
  /** For a Radix `onOpenChange` (click on a trigger, Escape, click outside). */
  setOpen: (open: boolean) => void;
  /** Spread on the element that previews on hover (and on keyboard focus). */
  triggerProps: {
    onPointerEnter: (event: React.PointerEvent) => void;
    onPointerLeave: (event: React.PointerEvent) => void;
    onFocus: (event: React.FocusEvent) => void;
    onBlur: () => void;
    onClick: (event: React.MouseEvent) => void;
  };
  /** Spread on the popover content, so the pointer can travel into it. */
  contentProps: {
    onPointerEnter: (event: React.PointerEvent) => void;
    onPointerLeave: (event: React.PointerEvent) => void;
    onOpenAutoFocus: (event: Event) => void;
  };
};

/**
 * Opens a popover while a mouse rests on its trigger (or the trigger has keyboard
 * focus) and closes it shortly after the pointer leaves both trigger and content.
 * A click on a trigger already previewing pins it open; touch is left alone.
 */
export function useHoverOpen(): HoverOpen {
  const [open, setOpenState] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Opened by hover or focus (closes on leave) rather than by a click (stays).
  const previewing = useRef(false);

  useEffect(() => () => clearTimeout(timer.current), []);

  function schedule(next: boolean, delay: number) {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      if (next || previewing.current) {
        previewing.current = next;
        setOpenState(next);
      }
    }, delay);
  }

  return {
    open,
    setOpen: (next) => {
      clearTimeout(timer.current);
      previewing.current = false;
      setOpenState(next);
    },
    triggerProps: {
      onPointerEnter: (event) => {
        if (isMouse(event) && !open) schedule(true, OPEN_DELAY_MS);
        else if (isMouse(event)) clearTimeout(timer.current);
      },
      onPointerLeave: (event) => {
        if (isMouse(event)) schedule(false, CLOSE_DELAY_MS);
      },
      onFocus: (event) => {
        if (
          !open &&
          event.target instanceof HTMLElement &&
          event.target.matches(":focus-visible")
        ) {
          schedule(true, 0);
        }
      },
      onBlur: () => schedule(false, 0),
      onClick: (event) => {
        // A preview the reader clicks becomes a regular, pinned popover.
        if (open && previewing.current) {
          event.preventDefault();
          clearTimeout(timer.current);
          previewing.current = false;
        }
      },
    },
    contentProps: {
      onPointerEnter: (event) => {
        if (isMouse(event)) clearTimeout(timer.current);
      },
      onPointerLeave: (event) => {
        if (isMouse(event)) schedule(false, CLOSE_DELAY_MS);
      },
      onOpenAutoFocus: (event) => {
        // A preview must not steal focus from the text being read.
        if (previewing.current) event.preventDefault();
      },
    },
  };
}
