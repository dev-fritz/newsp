import { useTranslation } from "react-i18next";

import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { useIdeaFormat } from "../hooks/use-idea-format";
import type { IdeaVersionSummary } from "../schemas/idea-response.schema";

type Props = {
  id: string;
  label: string;
  value: number;
  versions: readonly IdeaVersionSummary[];
  onChange: (version: number) => void;
};

/** Picks one version of an idea: "v2 · Deepened · 21 Oct". */
export function VersionSelect({ id, label, value, versions, onChange }: Props) {
  const { t } = useTranslation("ideas");
  const format = useIdeaFormat();
  return (
    <div className="flex w-full flex-col gap-1.5 sm:w-64">
      <Label htmlFor={id}>{label}</Label>
      <Select value={String(value)} onValueChange={(next) => onChange(Number(next))}>
        <SelectTrigger id={id} className="h-10 w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {versions.map((version) => (
            <SelectItem key={version.version} value={String(version.version)}>
              {t("compare.versionOption", {
                version: version.version,
                origin: t(`origin.${version.origin}`),
                date: format.date(version.created_at),
              })}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
