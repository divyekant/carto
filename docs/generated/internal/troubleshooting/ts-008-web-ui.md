# Troubleshooting: Web UI Issues

**ID:** ts-008
**Topic:** Web UI Issues
**Components:** `web/` — React + Vite + shadcn/ui, `server/` — static file serving

---

## Symptom: Blank Page at `localhost:8950`

**Cause:** The embedded SPA assets are missing, corrupted, or the Go binary was built without the `web/dist/` directory.

**Resolution:**

1. Verify the SPA was built before compiling the Go binary:
   ```bash
   cd web && npm run build && cd ..
   ```
   The `dist/` directory should contain `index.html` and associated JS/CSS assets.

2. Rebuild the Go binary:
   ```bash
   go build -o carto ./cmd/carto
   ```

3. Verify the `go:embed` directive in the server package references the correct path to the built assets.

4. Check the browser's developer tools (Console and Network tabs) for errors. A 404 on `index.html` confirms the embed is missing.

5. If running in development mode with Vite (`npm run dev`), access the Vite dev server directly (e.g., `http://localhost:5173`), not the Go server, unless the Vite proxy is configured.

---

## Symptom: API Errors in Browser Console

**Cause:** The frontend is making API calls that fail. Common scenarios:
- The Go server (`carto serve`) is not running.
- The frontend is hitting the wrong port.
- CORS is blocking requests during development.

**Resolution:**

1. Verify the server is running:
   ```bash
   curl http://localhost:8950/api/projects
   ```

2. If using the Vite dev server, check that the proxy configuration in `vite.config.ts` routes `/api/` to the Go server:
   ```js
   server: {
     proxy: {
       '/api': 'http://localhost:8950'
     }
   }
   ```

3. Check the browser's Network tab for the failing request. Look at the response status and body for details.

4. If the error is `Failed to fetch` or `net::ERR_CONNECTION_REFUSED`, the backend is not reachable. Start it with `carto serve`.

---

## Symptom: SSE Progress Not Updating

**Cause:** The SSE connection to the index endpoint failed to establish, or events are being buffered by a proxy.

**Resolution:**

1. Open the browser's Network tab and look for the SSE request (`/api/projects/{name}/index`). Check its status:
   - If it shows `pending`, the connection is open but no events have arrived yet. This is normal during slow phases.
   - If it shows an error status (4xx, 5xx), the request failed. Check the response body.
   - If it is missing, the frontend did not initiate the request. Check the browser console for JavaScript errors.

2. If events arrive but the UI does not update, there may be a rendering bug. Check the console for React errors.

3. If using a reverse proxy (nginx, Cloudflare), ensure it does not buffer SSE responses:
   ```nginx
   proxy_buffering off;
   proxy_cache off;
   ```

4. Try the SSE endpoint directly with curl to verify server-side behavior:
   ```bash
   curl -N -X POST http://localhost:8950/api/projects/myapp/index
   ```
   If events appear in curl but not in the browser, the issue is client-side.

---

## Symptom: Settings Not Saving

**Cause:** The `PUT /api/config` call returned an error, or the request was not sent.

**Resolution:**

1. Check the browser console for the API response. Look for error messages in the response body.

2. Verify the configuration values are valid. The API validates inputs and returns `400` for invalid values.

3. Check the server logs (stderr) for errors during the config update.

4. After saving, reload the Settings page to confirm the values persisted. If they reverted, the save may have silently failed.

5. Verify the server process has write access to the configuration store.

---

## Symptom: Dashboard Shows Stale Data

**Cause:** The Dashboard fetches project data on page mount. It does not poll or use WebSockets for live updates.

**Resolution:**

1. Refresh the browser page (F5 / Cmd+R) to re-fetch data.

2. Navigate away from the Dashboard and back to trigger a re-mount and data fetch.

3. This is by design. The only real-time data flow is the SSE stream on the Index page. Adding polling or WebSocket-based live updates to the Dashboard is a potential enhancement.

---

## Symptom: UI Layout Broken on Mobile

**Cause:** A component or custom style is not responsive, or the viewport meta tag is missing.

**Resolution:**

1. Verify `index.html` includes the viewport meta tag:
   ```html
   <meta name="viewport" content="width=device-width, initial-scale=1.0">
   ```

2. Check that shadcn/ui components are not overridden with fixed widths. The Table, for example, should use responsive column visibility.

3. Test in the browser's responsive mode (DevTools > Toggle Device Toolbar) to identify which component breaks.

---

## Quick Reference

| Symptom | First Check |
|---------|-------------|
| Blank page | Was `npm run build` run before `go build`? |
| API errors | Is `carto serve` running? Check `curl localhost:8950/api/projects`. |
| SSE not updating | Check Network tab for the SSE request status. |
| Settings not saving | Check browser console for the PUT response. |
| Stale dashboard | Refresh the page (data loads on mount only). |
| Broken mobile layout | Check viewport meta tag and responsive styles. |
