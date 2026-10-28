// POST { credential } with the ID token from Google Sign-In. Sets the session cookie.
import { fail, guardPost, json } from "../_lib/http.js";
import { verifyIdToken } from "../_lib/google.js";
import { cookieHeader, createSession } from "../_lib/session.js";

export async function POST(request) {
  const blocked = guardPost(request);
  if (blocked) return blocked;
  const { credential } = await request.json().catch(() => ({}));
  let user;
  try {
    user = await verifyIdToken(credential, process.env.GOOGLE_WEB_CLIENT_ID);
  } catch (err) {
    console.error("google sign-in:", err.message);
    return fail(500, "Sign-in is not available right now.");
  }
  if (!user) return fail(401, "Google did not accept that sign-in. Please try again.");
  return json({ email: user.email }, 200, { "set-cookie": cookieHeader(createSession(user)) });
}
