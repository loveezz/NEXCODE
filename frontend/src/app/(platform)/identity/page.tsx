import { Suspense } from "react";
import IdentitySettings from "@/components/identity-settings";
export const metadata = { title: "Группы и вход" };
export default function Page() {
  return (
    <Suspense>
      <IdentitySettings />
    </Suspense>
  );
}
