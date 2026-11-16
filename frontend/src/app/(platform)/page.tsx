import Dashboard from "@/components/dashboard";
import { catalogOnServer } from "@/lib/server";
export default async function Page() {
  return <Dashboard initial={await catalogOnServer()} />;
}
