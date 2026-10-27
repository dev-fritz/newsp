import { Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";

import { ErrorState, ListSkeleton } from "@/components/common/states";
import { Button } from "@/components/ui/button";
import { useCan } from "@/features/auth";

import { useSources } from "../api/sources-queries";
import { errorOf, FORBIDDEN } from "../lib/error-of";
import { SOURCE_PRESETS } from "../lib/source-presets";
import { SetupLayout } from "./setup-layout";
import { SourcePresetCard } from "./source-preset-card";
import { SourceSheet } from "./source-sheet";

/** Setup wizard, step 3: suggested sources, a custom one, and finish. */
export function SetupSourcesPage() {
  const { t } = useTranslation("admin");
  const navigate = useNavigate();
  const allowed = useCan("sources:manage");
  const sources = useSources();
  const [sheetOpen, setSheetOpen] = useState(false);

  const layoutProps = {
    step: 3 as const,
    title: t("setup.sources.title"),
    body: t("setup.sources.body"),
  };

  if (!allowed) {
    return (
      <SetupLayout {...layoutProps}>
        <ErrorState error={FORBIDDEN} />
      </SetupLayout>
    );
  }

  const names = new Set(sources.data?.map((s) => s.name));

  return (
    <SetupLayout {...layoutProps}>
      {sources.isPending && <ListSkeleton rows={3} />}
      {sources.isError && (
        <ErrorState error={errorOf(sources.error)} onRetry={() => void sources.refetch()} />
      )}
      {sources.data && (
        <>
          <section aria-labelledby="presets-title" className="flex flex-col gap-3">
            <h2 id="presets-title" className="text-sm font-semibold">
              {t("setup.sources.suggested")}
            </h2>
            <ul className="flex flex-col gap-2.5">
              {SOURCE_PRESETS.map((preset) => (
                <SourcePresetCard key={preset.id} preset={preset} added={names.has(preset.name)} />
              ))}
            </ul>
            <Button
              variant="outline"
              className="self-start pointer-coarse:h-11"
              onClick={() => setSheetOpen(true)}
            >
              <Plus aria-hidden />
              {t("setup.sources.custom")}
            </Button>
          </section>
          <output aria-live="polite" className="block text-sm text-muted-foreground">
            {t("setup.sources.count", { count: sources.data.length })}
          </output>
        </>
      )}
      <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
        <Button variant="ghost" asChild className="pointer-coarse:h-11">
          <Link to="/setup/ai">{t("common:actions.back")}</Link>
        </Button>
        <Button onClick={() => void navigate("/")} className="pointer-coarse:h-11">
          {t("setup.sources.finish")}
        </Button>
      </div>
      <SourceSheet open={sheetOpen} onOpenChange={setSheetOpen} />
    </SetupLayout>
  );
}
