import { Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";

import { AddProviderDialog } from "./add-provider-dialog";

/** Opens the add-provider dialog. */
export function AddProviderButton() {
  const { t } = useTranslation("admin");
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button onClick={() => setOpen(true)} className="pointer-coarse:h-11">
        <Plus aria-hidden />
        {t("ai.add.button")}
      </Button>
      <AddProviderDialog open={open} onOpenChange={setOpen} />
    </>
  );
}
