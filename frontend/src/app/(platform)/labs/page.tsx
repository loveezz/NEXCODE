import { Suspense } from "react";
import CatalogView from "@/components/catalog";
import { catalogOnServer } from "@/lib/server";
export const metadata = { title: "Лабораторные" };
export default async function Page() {
  const initial = await catalogOnServer();
  return (
    <Suspense>
      <CatalogView initial={initial} />
    </Suspense>
  );
}
