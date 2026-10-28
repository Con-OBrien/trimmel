// Tests for the website API in docs/api. Run:  node --test tests/site
// Google, Stripe and Upstash are replaced by an in-memory fake fetch.
import test from "node:test";
import assert from "node:assert/strict";
import { createHmac, generateKeyPairSync, createPublicKey, verify } from "node:crypto";

import { POST as googleSignIn } from "../../docs/api/auth/google.js";
import { POST as logout } from "../../docs/api/auth/logout.js";
import { GET as me } from "../../docs/api/me.js";
import { POST as checkout } from "../../docs/api/checkout.js";
import { POST as webhook } from "../../docs/api/webhooks/stripe.js";

const { privateKey, publicKey } = generateKeyPairSync("ed25519");
const seed = privateKey.export({ format: "der", type: "pkcs8" }).subarray(-32);
const pubRaw = publicKey.export({ format: "der", type: "spki" }).subarray(-32);

Object.assign(process.env, {
  SITE_URL: "https://trimmel.app",
  SESSION_SECRET: "x".repeat(40),
  GOOGLE_WEB_CLIENT_ID: "client-123",
  KV_REST_API_URL: "https://kv.test",
  KV_REST_API_TOKEN: "tok",
  STRIPE_SECRET_KEY: "sk_test_123",
  STRIPE_PRICE_ID: "price_plus",
  STRIPE_WEBHOOK_SECRET: "whsec_test",
  LICENCE_PRIVATE_KEY: Buffer.concat([seed, pubRaw]).toString("base64"),
  LICENCE_MAJOR: "1",
});

const kv = new Map();
let googleReply = {};
let checkoutReq = null;

globalThis.fetch = async (url, init = {}) => {
  url = String(url);
  const reply = (body, status = 200) => new Response(JSON.stringify(body), { status });
  if (url.startsWith("https://oauth2.googleapis.com/tokeninfo")) return reply(googleReply);
  if (url === "https://api.stripe.com/v1/checkout/sessions") {
    checkoutReq = { headers: init.headers, form: new URLSearchParams(String(init.body)) };
    return reply({ id: "cs_test_1", url: "https://pay.test/c/abc" });
  }
  if (url === "https://kv.test") {
    const [op, key, val, flag] = JSON.parse(init.body);
    if (op === "GET") return reply({ result: kv.get(key) ?? null });
    if (op === "SET") {
      if (flag === "NX" && kv.has(key)) return reply({ result: null });
      kv.set(key, val);
      return reply({ result: "OK" });
    }
  }
  throw new Error("unexpected fetch " + url);
};

const json = (path, body, headers = {}) =>
  new Request("https://trimmel.app" + path, {
    method: "POST",
    headers: { "content-type": "application/json", origin: "https://trimmel.app", ...headers },
    body: JSON.stringify(body),
  });

async function signIn(sub = "g-1", email = "Buyer@Example.com") {
  googleReply = { aud: "client-123", iss: "https://accounts.google.com", exp: String(Date.now() / 1000 + 600), sub, email, email_verified: "true" };
  const res = await googleSignIn(json("/api/auth/google", { credential: "c".repeat(50) }));
  assert.equal(res.status, 200);
  return res.headers.get("set-cookie").split(";")[0];
}

const getMe = (cookie) => me(new Request("https://trimmel.app/api/me", { headers: cookie ? { cookie } : {} }));

const NOW = () => Math.floor(Date.now() / 1000);
const sign = (body, t = NOW(), secret = "whsec_test") => `t=${t},v1=${createHmac("sha256", secret).update(`${t}.${body}`).digest("hex")}`;

function sessionEvent(over = {}, type = "checkout.session.completed") {
  return JSON.stringify({
    id: "evt_1",
    type,
    data: { object: { id: "cs_test_9001", object: "checkout.session", payment_status: "paid", livemode: false, payment_intent: "pi_9001", client_reference_id: "g-1", customer_details: { email: "Buyer@Example.com" }, metadata: { user_sub: "g-1", product: "trimmel-plus" }, ...over } },
  });
}
const hook = (body, sig) =>
  webhook(new Request("https://trimmel.app/api/webhooks/stripe", { method: "POST", body, headers: { "stripe-signature": sig ?? sign(body) } }));

test.beforeEach(() => kv.clear());

test("signed out: /api/me returns the Google client id", async () => {
  const body = await (await getMe()).json();
  assert.deepEqual(body, { signedIn: false, googleClientId: "client-123" });
});

test("sign-in sets a secure cookie and /api/me knows the user", async () => {
  googleReply = { aud: "client-123", iss: "accounts.google.com", exp: String(Date.now() / 1000 + 600), sub: "g-1", email: "Buyer@Example.com", email_verified: "true" };
  const res = await googleSignIn(json("/api/auth/google", { credential: "c".repeat(50) }));
  const set = res.headers.get("set-cookie");
  assert.match(set, /HttpOnly/);
  assert.match(set, /Secure/);
  assert.match(set, /SameSite=Lax/);
  const body = await (await getMe(set.split(";")[0])).json();
  assert.equal(body.signedIn, true);
  assert.equal(body.email, "buyer@example.com");
  assert.equal(body.plus, null);
});

test("sign-in rejects the wrong audience, unverified email and expired tokens", async () => {
  const good = { aud: "client-123", iss: "accounts.google.com", exp: String(Date.now() / 1000 + 600), sub: "g", email: "a@b.c", email_verified: "true" };
  for (const bad of [{ aud: "other" }, { email_verified: "false" }, { exp: "1" }, { iss: "evil.com" }]) {
    googleReply = { ...good, ...bad };
    const res = await googleSignIn(json("/api/auth/google", { credential: "c".repeat(50) }));
    assert.equal(res.status, 401, JSON.stringify(bad));
  }
});

test("a tampered or missing cookie is signed out", async () => {
  const cookie = await signIn();
  const tampered = cookie.slice(0, -3) + "AAA";
  assert.equal((await (await getMe(tampered)).json()).signedIn, false);
  assert.equal((await (await getMe("trimmel_session=junk")).json()).signedIn, false);
});

test("POST endpoints refuse other origins and non-JSON bodies", async () => {
  const cross = await googleSignIn(json("/api/auth/google", {}, { origin: "https://evil.example" }));
  assert.equal(cross.status, 403);
  const form = await logout(new Request("https://trimmel.app/api/auth/logout", { method: "POST", headers: { "content-type": "text/plain" }, body: "x" }));
  assert.equal(form.status, 415);
  assert.match((await logout(json("/api/auth/logout", {}))).headers.get("set-cookie"), /Max-Age=0/);
});

test("checkout needs a session and sends the account to Stripe", async () => {
  assert.equal((await checkout(json("/api/checkout", {}))).status, 401);
  const cookie = await signIn();
  const res = await checkout(json("/api/checkout", {}, { cookie }));
  assert.equal(res.status, 200);
  assert.equal((await res.json()).url, "https://pay.test/c/abc");
  const f = checkoutReq.form;
  assert.equal(checkoutReq.headers.authorization, "Bearer sk_test_123");
  assert.equal(checkoutReq.headers["stripe-version"], "2026-04-22.dahlia");
  assert.equal(f.get("mode"), "payment");
  assert.equal(f.get("managed_payments[enabled]"), "true");
  assert.equal(f.get("line_items[0][price]"), "price_plus");
  assert.equal(f.get("client_reference_id"), "g-1");
  assert.equal(f.get("metadata[user_sub]"), "g-1");
  assert.equal(f.get("metadata[product]"), "trimmel-plus");
  assert.equal(f.get("customer_email"), "buyer@example.com");
  assert.equal(f.get("success_url"), "https://trimmel.app/account.html?paid=1");
});

test("webhook rejects bad, wrong-secret, tampered and stale signatures", async () => {
  const body = sessionEvent();
  assert.equal((await hook(body, "t=1,v1=00")).status, 400);
  assert.equal((await hook(body, sign(body, NOW(), "whsec_other"))).status, 400);
  assert.equal((await hook(body + " ", sign(body))).status, 400);
  assert.equal((await hook(body, sign(body, NOW() - 3600))).status, 400);
  assert.equal((await hook(body, "")).status, 400);
  assert.equal(kv.size, 0);
});

test("webhook accepts any matching v1 signature in the header", async () => {
  const body = sessionEvent();
  assert.equal((await hook(body, sign(body) + ",v1=" + "ab".repeat(32))).status, 200);
  assert.ok(kv.has("order:cs_test_9001"));
});

test("a paid order stores a key that the account page shows, and the key verifies", async () => {
  const cookie = await signIn();
  assert.equal((await hook(sessionEvent())).status, 200);
  const body = await (await getMe(cookie)).json();
  assert.ok(body.plus.key.startsWith("TRIM1-"));
  assert.equal(body.plus.orderId, "cs_test_9001");

  // Same check the Go app makes: Ed25519 over the payload bytes.
  const [p, s] = body.plus.key.slice(6).split(".");
  const payload = Buffer.from(p, "base64url");
  assert.ok(verify(null, payload, createPublicKey(publicKey.export({ format: "pem", type: "spki" })), Buffer.from(s, "base64url")));
  const claims = JSON.parse(payload.toString());
  assert.equal(claims.id, "cs_test_9001");
  assert.equal(claims.email, "buyer@example.com");
  assert.equal(claims.account, "g-1");
  assert.equal(claims.major, 1);
  assert.equal(claims.devices, 3);
  assert.equal(claims.expires, undefined);
  assert.match(claims.issued, /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/);

  // Already owns Plus: no second checkout.
  assert.equal((await checkout(json("/api/checkout", {}, { cookie }))).status, 409);
});

test("webhook retries do not mint a second key", async () => {
  await hook(sessionEvent());
  const first = kv.get("order:cs_test_9001");
  assert.deepEqual(await (await hook(sessionEvent())).json(), { stored: false });
  assert.equal(kv.get("order:cs_test_9001"), first);
});

test("buying with another email still lands on the signed-in account", async () => {
  const cookie = await signIn("g-1", "me@example.com");
  await hook(sessionEvent({ customer_email: null, customer_details: { email: "other@example.com" } }));
  assert.ok((await (await getMe(cookie)).json()).plus);
});

test("with no account attached, the email still finds the order", async () => {
  await hook(sessionEvent({ client_reference_id: null, metadata: { product: "trimmel-plus" } }));
  const cookie = await signIn("g-2", "buyer@example.com");
  assert.ok((await (await getMe(cookie)).json()).plus);
});

test("other products, unpaid sessions and unrelated events are ignored", async () => {
  for (const over of [{ metadata: {} }, { metadata: { product: "other" } }, { payment_status: "unpaid" }]) {
    assert.deepEqual(await (await hook(sessionEvent(over))).json(), { ignored: true });
  }
  assert.deepEqual(await (await hook(sessionEvent({}, "customer.created"))).json(), { ignored: true });
  assert.equal(kv.size, 0);
});

test("a delayed payment that later succeeds issues the key", async () => {
  await hook(sessionEvent({ payment_status: "unpaid" }));
  assert.equal(kv.size, 0);
  await hook(sessionEvent({}, "checkout.session.async_payment_succeeded"));
  assert.ok(kv.has("order:cs_test_9001"));
});

test("live-mode sessions are not marked as test orders", async () => {
  await hook(sessionEvent({ livemode: true }));
  assert.equal(JSON.parse(kv.get("order:cs_test_9001")).testMode, false);
});

test("a partial refund keeps the key; a full refund hides it and allows a new purchase", async () => {
  const cookie = await signIn();
  await hook(sessionEvent());
  const charge = (refunded) => JSON.stringify({ id: "evt_2", type: "charge.refunded", data: { object: { id: "ch_1", payment_intent: "pi_9001", refunded } } });
  assert.deepEqual(await (await hook(charge(false))).json(), { ignored: true });
  assert.ok((await (await getMe(cookie)).json()).plus);
  assert.deepEqual(await (await hook(charge(true))).json(), { refunded: true });
  const me1 = await (await getMe(cookie)).json();
  assert.equal(me1.plus, null);
  assert.equal(me1.refunded, true);
  assert.equal((await checkout(json("/api/checkout", {}, { cookie }))).status, 200);
});
