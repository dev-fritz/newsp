import { useTranslation } from "react-i18next";

import { PROVIDER_KINDS } from "../lib/provider-kinds";
import type { ProviderKind } from "../types";
import { OptionCardGroup } from "./option-card-group";

type ProviderKindPickerProps = {
  value: ProviderKind;
  onChange: (kind: ProviderKind) => void;
  recommendLocal?: boolean;
};

/** Picks the provider kind; the local/OpenAI-compatible card covers Ollama and friends. */
export function ProviderKindPicker({ value, onChange, recommendLocal }: ProviderKindPickerProps) {
  const { t } = useTranslation("admin");
  return (
    <OptionCardGroup
      label={t("ai.form.kind")}
      value={value}
      onValueChange={(v) => {
        const kind = PROVIDER_KINDS.find((k) => k === v);
        if (kind) onChange(kind);
      }}
      minWidth={170}
      options={PROVIDER_KINDS.map((k) => ({
        value: k,
        title: t(`ai.kinds.${k}.title`),
        description: t(`ai.kinds.${k}.hint`),
        badge: recommendLocal && k === "openai_compatible" ? t("ai.form.recommended") : undefined,
      }))}
    />
  );
}
