import { useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { COMMON_TIMEZONES } from "@/features/profile/schemas/preferences-mapper";

const subscribe = (): (() => void) => () => undefined;
const browserZone = (): string => Intl.DateTimeFormat().resolvedOptions().timeZone;
const serverZone = (): string => "";

type TimezoneSelectProps = {
  id: string;
  value: string;
  onChange: (zone: string) => void;
};

/** Common zones plus the browser's and the saved one (SSR renders without the browser zone). */
export function TimezoneSelect({ id, value, onChange }: TimezoneSelectProps) {
  const { t } = useTranslation("profile");
  const browser = useSyncExternalStore(subscribe, browserZone, serverZone);
  const zones = new Set<string>([...(browser ? [browser] : []), value, ...COMMON_TIMEZONES]);
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger id={id} className="h-10 w-full pointer-coarse:h-11">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {[...zones].map((zone) => (
          <SelectItem key={zone} value={zone}>
            {zone === browser ? t("report.browserZone", { zone }) : zone}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
