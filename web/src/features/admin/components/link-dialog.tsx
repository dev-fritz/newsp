import { useTranslation } from "react-i18next";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import type { OneTimeLink as OneTimeLinkData } from "../types";
import { OneTimeLink } from "./one-time-link";

type LinkDialogProps = {
  title: string;
  description: string;
  link: OneTimeLinkData | null;
  onClose: () => void;
};

/** Shows a freshly created reset link once. */
export function LinkDialog({ title, description, link, onClose }: LinkDialogProps) {
  const { t } = useTranslation("admin");
  return (
    <Dialog open={link !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="gap-5 sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="font-serif text-2xl font-medium">{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {link && <OneTimeLink link={link} />}
        <p className="sr-only">{t("link.once")}</p>
      </DialogContent>
    </Dialog>
  );
}
