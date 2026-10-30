import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  return {
    plugins: [react()],
    server: {
      host: "0.0.0.0",
      port: 5173,
      strictPort: true,
      proxy: {
        "/health": {
          target: env.ARIMA_API_PROXY_TARGET || "http://127.0.0.1:8080",
          changeOrigin: true
        }
      }
    },
    build: {
      outDir: "dist",
      sourcemap: false
    }
  };
});
