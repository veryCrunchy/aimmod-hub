import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  envDir: "..",
  server: {
    port: 5173,
    proxy: process.env.OSU_SKIN_PREVIEW_API ? {
      '/api/osu/v1/playback/skins/': {
        target: process.env.OSU_SKIN_PREVIEW_API,
        changeOrigin: true,
      },
    } : undefined,
  }
});
