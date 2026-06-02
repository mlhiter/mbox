import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const apiTarget = process.env.MBOX_API_PROXY_TARGET || "http://127.0.0.1:18080";
const apiToken = process.env.MBOX_TOKEN || process.env.MBOX_API_TOKEN || "";
const webPort = Number(process.env.MBOX_WEB_PORT || "5174");
const trustedPrincipal = process.env.MBOX_TRUSTED_PRINCIPAL || "";
const trustedPrincipalType = process.env.MBOX_TRUSTED_PRINCIPAL_TYPE || "";

const proxyHeaders = {
  ...(apiToken ? { authorization: `Bearer ${apiToken}` } : {}),
  ...(trustedPrincipal ? { "x-mbox-principal": trustedPrincipal } : {}),
  ...(trustedPrincipalType ? { "x-mbox-principal-type": trustedPrincipalType } : {}),
};
const configuredProxyHeaders = Object.keys(proxyHeaders).length > 0 ? proxyHeaders : undefined;

export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: webPort,
    proxy: {
      "/healthz": {
        target: apiTarget,
        changeOrigin: true,
        headers: configuredProxyHeaders,
      },
      "/v1": {
        target: apiTarget,
        changeOrigin: true,
        ws: true,
        headers: configuredProxyHeaders,
      },
    },
  },
});
