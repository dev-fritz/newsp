import { useTranslation } from "react-i18next";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

import type { Competitor } from "../schemas/idea-response.schema";
import { IdeaSection } from "./idea-section";

/** Existing alternatives and the gap each leaves open. */
export function IdeaCompetitorsSection({ competitors }: { competitors: Competitor[] }) {
  const { t } = useTranslation("ideas");
  if (competitors.length === 0) return null;
  return (
    <IdeaSection title={t("sections.competitors")}>
      <div className="overflow-x-auto rounded-xl border border-border bg-card">
        <Table className="min-w-[560px]">
          <TableHeader>
            <TableRow>
              <TableHead>{t("competitors.name")}</TableHead>
              <TableHead>{t("competitors.strengths")}</TableHead>
              <TableHead>{t("competitors.gap")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {competitors.map((competitor, index) => (
              <TableRow key={`${index}-${competitor.name}`}>
                <TableCell className="align-top font-medium whitespace-normal">
                  {competitor.name}
                </TableCell>
                <TableCell className="align-top whitespace-normal text-muted-foreground">
                  {competitor.strengths}
                </TableCell>
                <TableCell className="align-top whitespace-normal">{competitor.gap}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </IdeaSection>
  );
}
