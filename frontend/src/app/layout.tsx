import type { Metadata } from "next";
import Providers from "@/components/providers";
import "./globals.css";
export const metadata: Metadata = {
  title: { default: "NEXCODE — Python Workspace", template: "%s · NEXCODE" },
  description:
    "Практические упражнения по Python. Пишите код, проверяйте решения и следите за прогрессом.",
};
export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="ru" suppressHydrationWarning>
      <body>
        <script
          dangerouslySetInnerHTML={{
            __html:
              "try{document.documentElement.dataset.theme=localStorage.getItem('nexcode-theme')||'light'}catch(e){}",
          }}
        />
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
