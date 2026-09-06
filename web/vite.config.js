import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { rosuBrowser } from "./scripts/rosu-browser";
export default defineConfig({
    plugins: [react(), tailwindcss(), rosuBrowser()],
    worker: { plugins: () => [rosuBrowser()] },
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
