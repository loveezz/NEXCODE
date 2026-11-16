import { Suspense } from "react";
import CatalogView from "@/components/catalog";
import { catalogOnServer } from "@/lib/server";
export const metadata = { title: "Мой прогресс" };
export default async function Page() {
  const initial = await catalogOnServer();
  return (
    <Suspense>
      <CatalogView progress initial={initial} />
    </Suspense>
  );
}
