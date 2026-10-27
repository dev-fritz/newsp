import { Controller, useWatch, type Control, type UseFormReturn } from "react-hook-form";
import { useTranslation } from "react-i18next";

import { FormField } from "@/components/common/form-field";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";

import {
  BLUESKY_MODES,
  BLUESKY_SORTS,
  DEVTO_TOPS,
  HN_LISTS,
  LOBSTERS_LISTS,
  MASTODON_MODES,
  MAX_GITHUB_DAYS,
  MAX_GITHUB_STARS,
  MAX_PH_DAYS,
  PH_ORDERS,
  REDDIT_SORTS,
  REDDIT_TIMES,
  secretMode,
  SOURCE_MAX_LIMIT,
  TN_STRATEGIES,
} from "../lib/source-kinds";
import type { SourceFormValues } from "../schemas/source.schema";
import { SelectField } from "./select-field";

type SourceConfigFieldsProps = { form: UseFormReturn<SourceFormValues> };

function EnumField({
  control,
  name,
  label,
  values,
  i18nPrefix,
}: {
  control: Control<SourceFormValues>;
  name:
    | "hnList"
    | "tnStrategy"
    | "redditSort"
    | "redditTime"
    | "lobstersList"
    | "devtoTop"
    | "mastodonMode"
    | "bskyMode"
    | "bskySort"
    | "phOrder";
  label: string;
  values: readonly string[];
  i18nPrefix: string;
}) {
  const { t } = useTranslation("admin");
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <SelectField
          label={label}
          value={field.value}
          onValueChange={field.onChange}
          options={values.map((v) => ({ value: v, label: t(`${i18nPrefix}.${v}`) }))}
          error={fieldState.error?.message}
        />
      )}
    />
  );
}

/** Write-only credential of the kinds that take one; blank keeps the saved one. */
function SecretField({ form }: SourceConfigFieldsProps) {
  const { t } = useTranslation("admin");
  const { register, control, formState } = form;
  const kind = useWatch({ control, name: "kind" });
  const bskyMode = useWatch({ control, name: "bskyMode" });
  const hasSecret = useWatch({ control, name: "hasSecret" });
  const clearSecret = useWatch({ control, name: "clearSecret" });
  const mode = secretMode(kind, { bskyMode });
  if (mode === "none") return null;

  const saved = hasSecret && !clearSecret;
  const hint = saved
    ? t("sources.secret.savedHint")
    : t(mode === "required" ? "sources.secret.requiredHint" : "sources.secret.optionalHint", {
        what: t(`sources.secret.hint.${kind}`),
      });
  return (
    <>
      <FormField
        label={t(`sources.secret.label.${kind}`)}
        type="password"
        autoComplete="new-password"
        autoCapitalize="none"
        autoCorrect="off"
        spellCheck={false}
        className="h-10 font-mono text-[13px]"
        placeholder={saved ? t("sources.secret.savedPlaceholder") : t("sources.secret.placeholder")}
        hint={hint}
        readOnly={clearSecret}
        error={formState.errors.secret?.message}
        {...register("secret")}
      />
      {mode === "optional" && hasSecret && (
        <Controller
          control={control}
          name="clearSecret"
          render={({ field }) => (
            <div className="flex min-h-6 items-center gap-2.5 pointer-coarse:min-h-11">
              <Checkbox
                id="source-clear-secret"
                checked={field.value}
                onCheckedChange={(next) => field.onChange(next === true)}
              />
              <Label htmlFor="source-clear-secret" className="font-normal">
                {t("sources.secret.remove")}
              </Label>
            </div>
          )}
        />
      )}
    </>
  );
}

/** Fields that depend on the connector kind of the source, then its credential if any. */
export function SourceConfigFields({ form }: SourceConfigFieldsProps) {
  return (
    <>
      <KindFields form={form} />
      <SecretField form={form} />
    </>
  );
}

function KindFields({ form }: SourceConfigFieldsProps) {
  const { t } = useTranslation("admin");
  const { register, control, formState } = form;
  const { errors } = formState;
  const kind = useWatch({ control, name: "kind" });
  const redditSort = useWatch({ control, name: "redditSort" });
  const mastodonMode = useWatch({ control, name: "mastodonMode" });
  const bskyMode = useWatch({ control, name: "bskyMode" });

  const text = (
    name: "lobstersTag" | "devtoTag" | "ghQuery" | "ghLanguage" | "ghTopic" | "phTopic",
    props: { label: string; placeholder: string; hint?: string },
  ) => (
    <FormField
      {...props}
      autoCapitalize="none"
      autoCorrect="off"
      spellCheck={false}
      error={errors[name]?.message}
      {...register(name)}
    />
  );

  const limit = (
    <FormField
      label={t("sources.form.limit")}
      type="number"
      inputMode="numeric"
      min={1}
      max={SOURCE_MAX_LIMIT[kind]}
      hint={t("sources.form.limitHint", { max: SOURCE_MAX_LIMIT[kind] })}
      error={errors.limit?.message}
      {...register("limit")}
    />
  );

  switch (kind) {
    case "hackernews":
      return (
        <>
          <EnumField
            control={control}
            name="hnList"
            label={t("sources.form.hnList")}
            values={HN_LISTS}
            i18nPrefix="sources.options.hnList"
          />
          {limit}
        </>
      );
    case "tabnews":
      return (
        <>
          <EnumField
            control={control}
            name="tnStrategy"
            label={t("sources.form.tnStrategy")}
            values={TN_STRATEGIES}
            i18nPrefix="sources.options.tnStrategy"
          />
          {limit}
        </>
      );
    case "reddit":
      return (
        <>
          <FormField
            label={t("sources.form.subreddit")}
            placeholder="SaaS"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            hint={t("sources.form.subredditHint")}
            error={errors.subreddit?.message}
            {...register("subreddit")}
          />
          <EnumField
            control={control}
            name="redditSort"
            label={t("sources.form.redditSort")}
            values={REDDIT_SORTS}
            i18nPrefix="sources.options.redditSort"
          />
          {redditSort === "top" && (
            <EnumField
              control={control}
              name="redditTime"
              label={t("sources.form.redditTime")}
              values={REDDIT_TIMES}
              i18nPrefix="sources.options.redditTime"
            />
          )}
          {limit}
        </>
      );
    case "youtube":
      return (
        <>
          <FormField
            label={t("sources.form.channelId")}
            placeholder="UCxxxxxxxxxxxxxxxxxxxxxx"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            className="h-10 font-mono text-[13px]"
            hint={t("sources.form.channelIdHint")}
            error={errors.channelId?.message}
            {...register("channelId")}
          />
          {limit}
        </>
      );
    case "rss":
      return (
        <>
          <FormField
            label={t("sources.form.url")}
            type="url"
            inputMode="url"
            placeholder="https://feeds.bbci.co.uk/news/technology/rss.xml"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            className="h-10 font-mono text-[13px]"
            hint={t("sources.form.urlHint")}
            error={errors.url?.message}
            {...register("url")}
          />
          {limit}
        </>
      );
    case "lobsters":
      return (
        <>
          <EnumField
            control={control}
            name="lobstersList"
            label={t("sources.form.lobstersList")}
            values={LOBSTERS_LISTS}
            i18nPrefix="sources.options.lobstersList"
          />
          {text("lobstersTag", {
            label: t("sources.form.lobstersTag"),
            placeholder: "ai",
            hint: t("sources.form.lobstersTagHint"),
          })}
          {limit}
        </>
      );
    case "devto":
      return (
        <>
          {text("devtoTag", {
            label: t("sources.form.devtoTag"),
            placeholder: "saas",
            hint: t("sources.form.devtoTagHint"),
          })}
          <EnumField
            control={control}
            name="devtoTop"
            label={t("sources.form.devtoTop")}
            values={DEVTO_TOPS}
            i18nPrefix="sources.options.devtoTop"
          />
          {limit}
        </>
      );
    case "mastodon":
      return (
        <>
          <FormField
            label={t("sources.form.mastodonInstance")}
            placeholder="mastodon.social"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            className="h-10 font-mono text-[13px]"
            hint={t("sources.form.mastodonInstanceHint")}
            error={errors.mastodonInstance?.message}
            {...register("mastodonInstance")}
          />
          <EnumField
            control={control}
            name="mastodonMode"
            label={t("sources.form.mastodonMode")}
            values={MASTODON_MODES}
            i18nPrefix="sources.options.mastodonMode"
          />
          {mastodonMode === "hashtag" ? (
            <FormField
              label={t("sources.form.mastodonHashtag")}
              placeholder="opensource"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              hint={t("sources.form.mastodonHashtagHint")}
              error={errors.mastodonHashtag?.message}
              {...register("mastodonHashtag")}
            />
          ) : (
            <FormField
              label={t("sources.form.mastodonAccount")}
              placeholder="user@other.host"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              className="h-10 font-mono text-[13px]"
              hint={t("sources.form.mastodonAccountHint")}
              error={errors.mastodonAccount?.message}
              {...register("mastodonAccount")}
            />
          )}
          {limit}
        </>
      );
    case "bluesky":
      return (
        <>
          <EnumField
            control={control}
            name="bskyMode"
            label={t("sources.form.bskyMode")}
            values={BLUESKY_MODES}
            i18nPrefix="sources.options.bskyMode"
          />
          {bskyMode === "author" && (
            <FormField
              label={t("sources.form.bskyActor")}
              placeholder="bsky.app"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              className="h-10 font-mono text-[13px]"
              hint={t("sources.form.bskyActorHint")}
              error={errors.bskyActor?.message}
              {...register("bskyActor")}
            />
          )}
          {bskyMode === "feed" && (
            <FormField
              label={t("sources.form.bskyFeed")}
              placeholder="at://did:plc:…/app.bsky.feed.generator/whats-hot"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              className="h-10 font-mono text-[13px]"
              hint={t("sources.form.bskyFeedHint")}
              error={errors.bskyFeed?.message}
              {...register("bskyFeed")}
            />
          )}
          {bskyMode === "search" && (
            <>
              <FormField
                label={t("sources.form.bskyQuery")}
                placeholder="indie hackers"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                hint={t("sources.form.bskyQueryHint")}
                error={errors.bskyQuery?.message}
                {...register("bskyQuery")}
              />
              <EnumField
                control={control}
                name="bskySort"
                label={t("sources.form.bskySort")}
                values={BLUESKY_SORTS}
                i18nPrefix="sources.options.bskySort"
              />
              <FormField
                label={t("sources.form.bskyIdentifier")}
                placeholder="you.bsky.social"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                className="h-10 font-mono text-[13px]"
                hint={t("sources.form.bskyIdentifierHint")}
                error={errors.bskyIdentifier?.message}
                {...register("bskyIdentifier")}
              />
            </>
          )}
          {limit}
        </>
      );
    case "github":
      return (
        <>
          {text("ghQuery", {
            label: t("sources.form.ghQuery"),
            placeholder: "agent framework",
            hint: t("sources.form.ghQueryHint"),
          })}
          {text("ghLanguage", {
            label: t("sources.form.ghLanguage"),
            placeholder: "Go",
          })}
          {text("ghTopic", {
            label: t("sources.form.ghTopic"),
            placeholder: "llm",
          })}
          <FormField
            label={t("sources.form.ghMinStars")}
            type="number"
            inputMode="numeric"
            min={1}
            max={MAX_GITHUB_STARS}
            hint={t("sources.form.rangeHint", { max: MAX_GITHUB_STARS })}
            error={errors.ghMinStars?.message}
            {...register("ghMinStars")}
          />
          <FormField
            label={t("sources.form.ghCreatedWithin")}
            type="number"
            inputMode="numeric"
            min={1}
            max={MAX_GITHUB_DAYS}
            hint={t("sources.form.rangeHint", { max: MAX_GITHUB_DAYS })}
            error={errors.ghCreatedWithin?.message}
            {...register("ghCreatedWithin")}
          />
          {limit}
        </>
      );
    case "producthunt":
      return (
        <>
          <EnumField
            control={control}
            name="phOrder"
            label={t("sources.form.phOrder")}
            values={PH_ORDERS}
            i18nPrefix="sources.options.phOrder"
          />
          {text("phTopic", {
            label: t("sources.form.phTopic"),
            placeholder: "artificial-intelligence",
            hint: t("sources.form.phTopicHint"),
          })}
          <FormField
            label={t("sources.form.phPostedWithin")}
            type="number"
            inputMode="numeric"
            min={1}
            max={MAX_PH_DAYS}
            hint={t("sources.form.rangeHint", { max: MAX_PH_DAYS })}
            error={errors.phPostedWithin?.message}
            {...register("phPostedWithin")}
          />
          {limit}
        </>
      );
  }
}
