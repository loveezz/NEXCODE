"use client";
import { usePathname, useSearchParams } from "next/navigation";
import ExerciseEditor from "./editor";
import { ErrorState } from "./common";
export default function ExercisePage() {
  const search = useSearchParams();
  const pathname = usePathname();
  const id = search.get("id") || pathname.match(/\/exercise\/(\d+)/)?.[1];
  if (!id || !/^\d+$/.test(id))
    return <ErrorState error={new Error("Выберите упражнение в каталоге.")} />;
  return <ExerciseEditor key={id} exerciseId={Number(id)} />;
}
