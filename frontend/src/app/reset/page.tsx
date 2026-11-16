import { redirect } from "next/navigation";
export const metadata = { title: "Восстановление пароля" };
export default function Page() {
  redirect("/staff/login");
}
