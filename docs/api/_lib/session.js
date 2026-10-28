// Signed session cookie: base64url(JSON payload) + "." + base64url(HMAC-SHA256). No server-side
// session store; the payload carries the Google account id, the email and an expiry.
import { createHmac, timingSafeEqual } from "node:crypto";
import { readCookie } from "./http.js";

export const COOKIE = "trimmel_session";
const MAX_AGE = 30 * 24 * 60 * 60; // seconds

const b64u = (buf) => Buffer.from(buf).toString("base64url");
const mac = (data) => createHmac("sha256", process.env.SESSION_SECRET || "").update(data).digest();

function secretOK() {
  return (process.env.SESSION_SECRET || "").length >= 32;
}

export function createSession(user, now = Date.now()) {
  if (!secretOK()) throw new Error("SESSION_SECRET must be at least 32 characters");
  const payload = b64u(JSON.stringify({ sub: user.sub, email: user.email, exp: Math.floor(now / 1000) + MAX_AGE }));
  return `${payload}.${b64u(mac(payload))}`;
}

export function readSession(request, now = Date.now()) {
  if (!secretOK()) return null;
  const token = readCookie(request, COOKIE);
  const [payload, sig] = token.split(".");
  if (!payload || !sig) return null;
  const want = mac(payload);
  const got = Buffer.from(sig, "base64url");
  if (got.length !== want.length || !timingSafeEqual(got, want)) return null;
  try {
    const s = JSON.parse(Buffer.from(payload, "base64url").toString());
    if (!s.sub || !s.email || s.exp * 1000 <= now) return null;
    return { sub: String(s.sub), email: String(s.email) };
  } catch {
    return null;
  }
}

export const cookieHeader = (token) =>
  `${COOKIE}=${token}; Path=/; Max-Age=${MAX_AGE}; HttpOnly; Secure; SameSite=Lax`;

export const clearCookieHeader = () => `${COOKIE}=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax`;
