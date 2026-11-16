import type { NextConfig } from "next";
const pages = process.env.GITHUB_PAGES === "true";
const config: NextConfig = {
  output: pages ? "export" : "standalone",
  distDir: pages ? "out" : ".next",
  basePath: process.env.NEXT_PUBLIC_BASE_PATH || "",
  trailingSlash: pages,
  images: { unoptimized: pages },
  poweredByHeader: false,
  ...(!pages
    ? {
        async rewrites() {
          return [
            { source: "/exercise/:id", destination: "/exercise?id=:id" },
            {
              source: "/api/:path*",
              destination: `${process.env.BACKEND_URL || "http://127.0.0.1:8080"}/api/:path*`,
            },
          ];
        },
      }
    : {}),
  ...(!pages
    ? {
        async headers() {
          return [
            {
              source: "/:path*",
              headers: [
                { key: "X-Frame-Options", value: "DENY" },
                { key: "X-Content-Type-Options", value: "nosniff" },
                { key: "Referrer-Policy", value: "same-origin" },
                {
                  key: "Permissions-Policy",
                  value: "camera=(), microphone=(), geolocation=()",
                },
                {
                  key: "Content-Security-Policy",
                  value:
                    "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; worker-src 'self' blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
                },
              ],
            },
          ];
        },
      }
    : {}),
};
export default config;
