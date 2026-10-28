// Small helpers shared by the API functions. Files starting with "_" are not routes.

export const siteURL = () => (process.env.SITE_URL || "https://trimmel.app").replace(/\/$/, "");

export function json(body, status = 200, headers = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", "cache-control": "no-store", ...headers },
  });
}

export const fail = (status, message) => json({ error: message }, status);

// guardPost rejects cross-site and form-style POSTs. Returns a Response to send, or null to go on.
// Requiring a JSON content type stops plain HTML forms, and the Origin check stops other sites.
export function guardPost(request) {
  const origin = request.headers.get("origin");
  if (origin && origin !== siteURL()) return fail(403, "Request not allowed from this site.");
  const type = request.headers.get("content-type") || "";
  if (!type.toLowerCase().startsWith("application/json")) return fail(415, "Expected JSON.");
  return null;
}

export function readCookie(request, name) {
  for (const part of (request.headers.get("cookie") || "").split(";")) {
    const i = part.indexOf("=");
    if (i > 0 && part.slice(0, i).trim() === name) return part.slice(i + 1).trim();
  }
  return "";
}
