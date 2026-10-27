import { useTranslation } from "react-i18next";

import { Table, TableBody, TableHead, TableHeader, TableRow } from "@/components/ui/table";

import type { InstanceSettings, Provider } from "../types";
import { ModelRow } from "./model-row";

type ModelsTableProps = {
  providers: Provider[];
  settings: InstanceSettings | undefined;
};

/** Every registered model with its capability, usage and enabled switch. */
export function ModelsTable({ providers, settings }: ModelsTableProps) {
  const { t } = useTranslation("admin");
  const rows = providers.flatMap((p) => p.models.map((model) => ({ model, provider: p })));
  return (
    <section aria-labelledby="models-title" className="flex flex-col gap-3.5">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <h2 id="models-title" className="font-serif text-[26px] font-medium">
          {t("ai.models.title")}
        </h2>
        <span className="text-xs text-subtle-foreground">{t("ai.models.hint")}</span>
      </div>
      {rows.length === 0 ? (
        <p className="rounded-2xl border border-dashed border-border-strong p-5 text-sm text-muted-foreground">
          {t("ai.models.empty")}
        </p>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-border bg-card">
          <Table className="min-w-[760px]">
            <TableHeader>
              <TableRow>
                <TableHead className="kicker">{t("ai.models.model")}</TableHead>
                <TableHead className="kicker">{t("ai.models.provider")}</TableHead>
                <TableHead className="kicker">{t("ai.models.use")}</TableHead>
                <TableHead className="kicker text-right">{t("ai.models.tokens")}</TableHead>
                <TableHead className="kicker text-right">{t("ai.models.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map(({ model, provider }) => (
                <ModelRow
                  key={model.id}
                  model={model}
                  providerName={provider.name}
                  settings={settings}
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </section>
  );
}
