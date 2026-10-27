import { useTranslation } from "react-i18next";

import type { ModelOption } from "../lib/model-options";
import { SelectField } from "./select-field";

type DefaultModelsPickerProps = {
  chatOptions: ModelOption[];
  embeddingOptions: ModelOption[];
  chatId: string;
  embeddingId: string;
  onChatChange: (id: string) => void;
  onEmbeddingChange: (id: string) => void;
};

/** Wizard pickers: the model that writes the texts and the embedding model. */
export function DefaultModelsPicker({
  chatOptions,
  embeddingOptions,
  chatId,
  embeddingId,
  onChatChange,
  onEmbeddingChange,
}: DefaultModelsPickerProps) {
  const { t } = useTranslation("admin");
  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4">
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(200px,100%),1fr))] gap-3.5">
        <SelectField
          label={t("setup.ai.chatModel")}
          value={chatId}
          onValueChange={onChatChange}
          options={chatOptions}
          placeholder={t("ai.settings.pick")}
          hint={chatOptions.length === 0 ? t("setup.ai.noChat") : undefined}
          disabled={chatOptions.length === 0}
        />
        <SelectField
          label={t("setup.ai.embeddingModel")}
          value={embeddingId}
          onValueChange={onEmbeddingChange}
          options={embeddingOptions}
          placeholder={t("ai.settings.pick")}
          hint={embeddingOptions.length === 0 ? t("setup.ai.noEmbedding") : undefined}
          disabled={embeddingOptions.length === 0}
        />
      </div>
    </div>
  );
}
