import { Suspense } from "react";
import ExercisePage from "@/components/exercise-page";
export const metadata = { title: "Редактор Python" };
export default function Page() {
  return (
    <Suspense>
      <ExercisePage />
    </Suspense>
  );
}
