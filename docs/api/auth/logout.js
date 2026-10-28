// POST. Clears the session cookie.
import { guardPost, json } from "../_lib/http.js";
import { clearCookieHeader } from "../_lib/session.js";

export async function POST(request) {
  const blocked = guardPost(request);
  if (blocked) return blocked;
  return json({ ok: true }, 200, { "set-cookie": clearCookieHeader() });
}
