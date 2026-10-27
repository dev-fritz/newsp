import type { Config } from "@react-router/dev/config";

export default {
  appDirectory: "src",
  // SSR is required: the server half of the app is the BFF that holds the session.
  ssr: true,
} satisfies Config;
