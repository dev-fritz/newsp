import { FileText, Sparkles } from "lucide-react";
import { useSyncExternalStore } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";

import { PageHeader } from "@/components/common/page-header";
import { Button } from "@/components/ui/button";
import { useCurrentUser } from "@/features/auth";
import { useIdeaGeneration } from "@/features/ideas";

import { dayPart, firstName, formatKickerDate } from "../lib/greeting";

const MINUTE_MS = 60_000;

function subscribeToClock(onChange: () => void): () => void {
  const timer = setInterval(onChange, MINUTE_MS);
  return () => clearInterval(timer);
}

function clockMinute(): number {
  return Math.floor(Date.now() / MINUTE_MS);
}

/** Greeting kicker with today's date and the user's name, plus the two generate actions. */
export function HomeHeader() {
  const { t, i18n } = useTranslation("home");
  const user = useCurrentUser();
  const { openDialog } = useIdeaGeneration();
  // The date depends on the visitor's clock and time zone: it is only known on the client
  // (null on the server), so hydration does not mismatch.
  const minute = useSyncExternalStore(subscribeToClock, clockMinute, () => null);
  const now = minute === null ? null : new Date(minute * MINUTE_MS);
  const name = firstName(user.name);
  return (
    <PageHeader
      kicker={now ? formatKickerDate(now, i18n.language) : undefined}
      title={t(`greeting.${now ? dayPart(now) : "neutral"}`, { name })}
      description={t("subtitle")}
      actions={
        <>
          <Button asChild variant="outline" className="h-10 pointer-coarse:h-11">
            <Link to="/reports">
              <FileText aria-hidden />
              {t("actions.generateReport")}
            </Link>
          </Button>
          <Button
            type="button"
            className="h-10 pointer-coarse:h-11"
            aria-haspopup="dialog"
            onClick={() => openDialog()}
          >
            <Sparkles aria-hidden />
            {t("actions.generateIdeas")}
          </Button>
        </>
      }
    />
  );
}
