import { HomeHeader } from "./home-header";
import { HomeIdeas } from "./home-ideas";
import { HomeReport } from "./home-report";
import { HomeTrending } from "./home-trending";

/** The `/` screen: lead report, recent ideas and what is trending. Blocks fail independently. */
export function HomeScreen() {
  return (
    <div className="flex min-w-0 flex-col gap-8">
      <HomeHeader />
      <hr className="m-0 border-t border-border-strong" />
      <div className="grid min-w-0 gap-10 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex min-w-0 flex-col gap-10">
          <HomeReport />
          <HomeTrending />
        </div>
        <HomeIdeas />
      </div>
    </div>
  );
}
