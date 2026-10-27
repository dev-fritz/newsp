import { useTranslation } from "react-i18next";

import { SetupSteps } from "@/features/auth";

type SetupLayoutProps = {
  step: 2 | 3;
  title: string;
  body: string;
  aside?: React.ReactNode;
  children: React.ReactNode;
};

/** Guided layout of the setup wizard: progress, explanation on the left, form on the right. */
export function SetupLayout({ step, title, body, aside, children }: SetupLayoutProps) {
  const { t } = useTranslation("admin");
  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-10">
      <SetupSteps current={step} />
      <div className="flex flex-wrap gap-x-14 gap-y-8">
        <div className="flex min-w-0 flex-[1_1_280px] flex-col gap-4">
          <span className="kicker">{t("setup.stepOf", { current: step, total: 3 })}</span>
          <h1 className="font-serif text-4xl leading-[1.08] font-medium text-balance">{title}</h1>
          <p className="text-[15.5px] leading-relaxed text-muted-foreground">{body}</p>
          {aside}
        </div>
        <div className="flex min-w-0 flex-[999_1_460px] flex-col gap-6">{children}</div>
      </div>
    </div>
  );
}
