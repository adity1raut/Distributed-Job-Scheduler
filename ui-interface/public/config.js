// Runtime config, read by src/api/client.js before falling back to the
// build-time VITE_API_URL. This checked-in copy is the default used by
// `npm run dev` / `npm run preview` and by the built dist/ before it's
// containerized.
//
// Inside the Docker image this exact file is regenerated at container
// startup from the VITE_API_URL environment variable — see
// docker-entrypoint.d/40-render-config.sh and config.template.js — so the
// same built image can be pointed at a different API per environment
// without a rebuild.
window.__APP_CONFIG__ = { API_URL: 'http://localhost:8080' }
