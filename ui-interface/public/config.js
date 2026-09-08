// Runtime configuration placeholder.
//
// Vite inlines import.meta.env.VITE_* at BUILD time, which would mean one
// image per environment. The container entrypoint overwrites this file on
// startup from the API_URL env var, so a single image can be built once and
// promoted through staging and production unchanged.
//
// API_URL: null here means "not configured at runtime" — the dev server has
// no /api proxy, so src/api/client.js falls back to VITE_API_URL from .env.
window.__APP_CONFIG__ = { API_URL: null };
