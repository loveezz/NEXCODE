import Shell from "@/components/shell";
export default function PlatformLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <Shell>{children}</Shell>;
}
