// Checks a Google Sign-In ID token by asking Google to validate it (tokeninfo), then confirms it was
// issued for this site's client ID and carries a verified email.

const ISSUERS = new Set(["accounts.google.com", "https://accounts.google.com"]);

export async function verifyIdToken(credential, clientId, now = Date.now()) {
  if (!clientId) throw new Error("GOOGLE_WEB_CLIENT_ID is not set");
  if (typeof credential !== "string" || credential.length < 20 || credential.length > 4096) return null;
  const res = await fetch("https://oauth2.googleapis.com/tokeninfo?id_token=" + encodeURIComponent(credential));
  if (!res.ok) return null;
  const t = await res.json();
  if (t.aud !== clientId || !ISSUERS.has(t.iss)) return null;
  if (Number(t.exp) * 1000 <= now) return null;
  if (!t.sub || !t.email || String(t.email_verified) !== "true") return null;
  return { sub: String(t.sub), email: String(t.email).toLowerCase() };
}
