import { defineConfig } from "vite";
import react, { reactCompilerPreset } from "@vitejs/plugin-react";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";

// The API (and the SSO routes /auth/*) are proxied, so the browser talks to one origin (:5173) and the
// HttpOnly fin_session cookie is same-origin.
const api = "http://localhost:8080";

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    tanstackRouter(),
    react(),
    tailwindcss(),
    babel({ presets: [reactCompilerPreset()] }),
  ],
  resolve: {
    alias: {
      "@": `${import.meta.dirname}/src`,
    },
  },
  server: {
    proxy: {
      "/api": api,
      "/auth": api,
    },
  },
});
