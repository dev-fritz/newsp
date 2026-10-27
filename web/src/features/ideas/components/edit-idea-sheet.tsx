import { useTranslation } from "react-i18next";

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";

import type { Idea } from "../schemas/idea-response.schema";
import { EditIdeaForm } from "./edit-idea-form";

type Props = { idea: Idea; open: boolean; onOpenChange: (open: boolean) => void };

/** Side sheet with the edit form; the form is mounted only while open so it starts fresh. */
export function EditIdeaSheet({ idea, open, onOpenChange }: Props) {
  const { t } = useTranslation("ideas");
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full gap-0 data-[side=right]:w-full data-[side=right]:sm:max-w-2xl">
        <SheetHeader className="pr-12">
          <SheetTitle className="font-serif text-2xl font-medium">{t("edit.heading")}</SheetTitle>
          <SheetDescription>
            {t("edit.description", { version: idea.current_version })}
          </SheetDescription>
        </SheetHeader>
        {open && <EditIdeaForm idea={idea} onDone={() => onOpenChange(false)} />}
      </SheetContent>
    </Sheet>
  );
}
