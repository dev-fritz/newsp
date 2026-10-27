import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { ErrorState, ListSkeleton } from "@/components/common/states";
import { Button } from "@/components/ui/button";
import { useCan } from "@/features/auth";

import type { UpdateSettingsBody } from "../api/ai-api";
import { useProviders, useSettings, useTestProvider, useUpdateSettings } from "../api/ai-queries";
import { useErrorToast } from "../hooks/use-error-toast";
import { errorOf, FORBIDDEN } from "../lib/error-of";
import { modelOptions } from "../lib/model-options";
import { OLLAMA_BASE_URL } from "../lib/provider-kinds";
import type { ConnectionTest } from "../types";
import { DefaultModelsPicker } from "./default-models-picker";
import { ProviderConnection } from "./provider-connection";
import { ProviderForm } from "./provider-form";
import { SetupLayout } from "./setup-layout";

/** Setup wizard, step 2: connect a provider, pick models, choose the defaults. */
export function SetupAiPage() {
  const { t } = useTranslation("admin");
  const navigate = useNavigate();
  const toastError = useErrorToast();
  const allowed = useCan("providers:manage");
  const canSettings = useCan("settings:manage");
  const providers = useProviders();
  const settings = useSettings(canSettings);
  const update = useUpdateSettings();
  const test = useTestProvider();

  const [adding, setAdding] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [created, setCreated] = useState<{ id: string; test: ConnectionTest | null } | null>(null);
  const [choice, setChoice] = useState<{ chat?: string; embedding?: string }>({});

  const layoutProps = {
    step: 2 as const,
    title: t("setup.ai.title"),
    body: t("setup.ai.body"),
    aside: (
      <div className="flex flex-col gap-2.5 rounded-xl bg-muted p-4 text-[13.5px] leading-normal">
        <strong className="font-semibold">{t("setup.ai.composeTitle")}</strong>
        <span className="text-muted-foreground">
          {t("setup.ai.composeBody", { url: OLLAMA_BASE_URL })}
        </span>
      </div>
    ),
  };

  if (!allowed) {
    return (
      <SetupLayout {...layoutProps}>
        <ErrorState error={FORBIDDEN} />
      </SetupLayout>
    );
  }
  if (providers.isPending) {
    return (
      <SetupLayout {...layoutProps}>
        <ListSkeleton rows={2} />
      </SetupLayout>
    );
  }
  if (providers.isError) {
    return (
      <SetupLayout {...layoutProps}>
        <ErrorState error={errorOf(providers.error)} onRetry={() => void providers.refetch()} />
      </SetupLayout>
    );
  }

  const list = providers.data;
  const active = list.find((p) => p.id === activeId) ?? list[0];
  const showForm = adding || !active;

  const chatOptions = modelOptions(list, "chat");
  const embeddingOptions = modelOptions(list, "embedding");
  const chatId =
    choice.chat ?? settings.data?.default_report_model_id ?? chatOptions[0]?.value ?? "";
  const embeddingId =
    choice.embedding ?? settings.data?.embedding_model_id ?? embeddingOptions[0]?.value ?? "";

  async function proceed() {
    const s = settings.data;
    const body: UpdateSettingsBody = {};
    if (chatId && chatId !== s?.default_report_model_id) body.default_report_model_id = chatId;
    if (chatId && !s?.default_idea_model_id) body.default_idea_model_id = chatId;
    if (embeddingId && embeddingId !== s?.embedding_model_id) body.embedding_model_id = embeddingId;
    try {
      if (Object.keys(body).length > 0) await update.mutateAsync(body);
      void navigate("/setup/sources");
    } catch (error) {
      toastError(error);
    }
  }

  return (
    <SetupLayout {...layoutProps}>
      {showForm ? (
        <ProviderForm
          recommendLocal
          onCancel={active ? () => setAdding(false) : undefined}
          onConnected={(p, result) => {
            setActiveId(p.id);
            setCreated({ id: p.id, test: result });
            setAdding(false);
          }}
        />
      ) : (
        <>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-sm font-semibold">{active.name}</h2>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setAdding(true)}
              className="pointer-coarse:h-11"
            >
              {t("setup.ai.addAnother")}
            </Button>
          </div>
          <ProviderConnection
            provider={active}
            result={
              test.variables === active.id && test.data
                ? test.data
                : created?.id === active.id
                  ? created.test
                  : null
            }
            pending={test.isPending}
            onRetest={() => test.mutate(active.id, { onError: toastError })}
          />
          <DefaultModelsPicker
            chatOptions={chatOptions}
            embeddingOptions={embeddingOptions}
            chatId={chatId}
            embeddingId={embeddingId}
            onChatChange={(id) => setChoice((c) => ({ ...c, chat: id }))}
            onEmbeddingChange={(id) => setChoice((c) => ({ ...c, embedding: id }))}
          />
          {embeddingOptions.length === 0 && (
            <p className="text-xs text-subtle-foreground">{t("setup.ai.embeddingWarning")}</p>
          )}
        </>
      )}
      <div className="flex flex-wrap justify-end gap-2 pt-1">
        <Button
          variant="ghost"
          onClick={() => void navigate("/setup/sources")}
          className="pointer-coarse:h-11"
        >
          {t("setup.ai.later")}
        </Button>
        <Button
          onClick={() => void proceed()}
          disabled={chatOptions.length === 0 || update.isPending || showForm}
          className="pointer-coarse:h-11"
        >
          {t("common:actions.continue")}
        </Button>
      </div>
    </SetupLayout>
  );
}
