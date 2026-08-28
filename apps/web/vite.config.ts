import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const productionWsURL = 'wss://zutto-pccom-prototype.onrender.com/ws';

export default defineConfig(({ command, mode }) => {
  // Vercel may expose an optional env var as an empty process variable. Vite
  // gives existing process.env values precedence over .env.production, so an
  // empty dashboard value can otherwise mask the repository production value.
  // Fill it before Vite resolves import.meta.env for the production build.
  if (command === 'build' && mode === 'production' && !(process.env.VITE_WS_URL ?? '').trim()) {
    process.env.VITE_WS_URL = productionWsURL;
  }

  return {
    plugins: [react()],
    server: { port: 5173 },
  };
});
