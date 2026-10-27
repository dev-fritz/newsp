import { Sparkles } from "lucide-react";
import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";

import { PageHeader } from "@/components/common/page-header";
import { Button } from "@/components/ui/button";
import { IdeasList } from "@/features/ideas/components/ideas-list";
import { IdeasToolbar } from "@/features/ideas/components/ideas-toolbar";
import { useIdeaGeneration } from "@/features/ideas/hooks/use-idea-generation";
import { ideaListParamsSchema, type IdeaListParams } from "@/features/ideas/schemas/idea.schema";
import { pageTitle } from "@/lib/meta";

import type { Route } from "./+types/ideas";

export function meta({ matches }: Route.MetaArgs) {
  return pageTitle(matches, "ideas");
}

export default function Ideas() {
  const { t } = useTranslation("ideas");
  const [params, setParams] = useSearchParams();
  const { status, q } = ideaListParamsSchema.parse({
    status: params.get("status") ?? undefined,
    q: params.get("q") ?? "",
  });
  const { running, openDialog } = useIdeaGeneration();

  // Legacy links (old `generate` param) open the dialog once and drop the param.
  const handledLegacy = useRef(false);
  useEffect(() => {
    if (handledLegacy.current || params.get("generate") !== "1") return;
    handledLegacy.current = true;
    openDialog();
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous);
        next.delete("generate");
        return next;
      },
      { replace: true },
    );
  }, [params, setParams, openDialog]);

  function update(next: Partial<IdeaListParams>) {
    setParams(
      (previous) => {
        const merged = new URLSearchParams(previous);
        const value = { status, q, ...next };
        if (value.status === "active") merged.delete("status");
        else merged.set("status", value.status);
        if (value.q === "") merged.delete("q");
        else merged.set("q", value.q);
        return merged;
      },
      { replace: true },
    );
  }

  return (
    <div className="flex w-full flex-col gap-[26px]">
      <PageHeader
        kicker={t("list.kicker")}
        title={t("list.title")}
        description={t("list.intro")}
        actions={
          <Button
            type="button"
            className="h-10"
            aria-haspopup="dialog"
            onClick={() => openDialog()}
          >
            {running ? (
              <>
                <span
                  aria-hidden
                  className="size-[7px] animate-pulse rounded-full bg-primary-foreground motion-reduce:animate-none"
                />
                {t("dialog.generating")}
              </>
            ) : (
              <>
                <Sparkles aria-hidden />
                {t("dialog.open")}
              </>
            )}
          </Button>
        }
      />
      <IdeasToolbar
        status={status}
        query={q}
        onStatusChange={(next) => update({ status: next })}
        onQueryChange={(next) => update({ q: next })}
      />
      <IdeasList status={status} query={q} />
    </div>
  );
}
