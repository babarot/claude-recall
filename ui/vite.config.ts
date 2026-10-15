import { defineConfig } from "vite";
import preact from "@preact/preset-vite";

// The recall ui API calls go to: the one mise run dev starts for this
// worktree, or else the usual one
const RECALL = process.env.RECALL_API ?? "http://127.0.0.1:6276";

export default defineConfig({
  plugins: [preact()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": RECALL,
    },
  },
});
