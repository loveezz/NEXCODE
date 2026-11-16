import { Suspense } from "react";
import HistoryView from "@/components/history";
export const metadata = { title: "История отправок" };
export default function Page() {
  return (
    <Suspense>
      <HistoryView />
    </Suspense>
  );
}
