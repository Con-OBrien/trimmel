// Tests for the website API in docs/api. Run:  node --test tests/site
// Google, Lemon Squeezy and Upstash are replaced by an in-memory fake fetch.
import test from "node:test";
import assert from "node:assert/strict";
import { createHmac, generateKeyPairSync, createPublicKey, verify } from "node:crypto";

import { POST as googleSignIn } from "../../docs/api/auth/google.js";
import { POST as logout } from "../../docs/api/auth/logout.js";
import { GET as me } from "../../docs/api/me.js";
import { POST as checkout } from "../../docs/api/checkout.js";
import { POST as webhook } from "../../docs/api/webhooks/lemonsqueezy.js";

const { privateKey, publicKey } = generateKeyPairSync("ed25519");
const seed = privateKey.export({ format: "der", type: "pkcs8" }).subarray(-32);
const pubRaw = publicKey.export({ format: "der", type: "spki" }).subarray(-32);

Object.assign(process.env, {
  SITE_URL: "https://trimmel.app",
  SESSION_SECRET: "x".repeat(40),
  GOOGLE_WEB_CLIENT_ID: "client-123",
  KV_REST_API_URL: "https://kv.test",
  KV_REST_API_TOKEN: "tok",
  LS_API_KEY: "ls-key",
  LS_STORE_ID: "11",
  LS_VARIANT_ID: "22",
  LS_WEBHOOK_SECRET: "whsec",
  LICENCE_PRIVATE_KEY: Buffer.concat([seed, pubRaw]).toString("base64"),
  LICENCE_MAJOR: "1",
});

const kv = new Map();
let googleReply = {};
let checkoutBody = null;

globalThis.fetch = async (url, init = {}) => {
  url = String(url);
  const reply = (body, status = 200) => new Response(JSON.stringify(body), { status });
  if (url.startsWith("https://oauth2.googleapis.com/tokeninfo")) return reply(googleReply);
  if (url === "https://api.lemonsqueezy.com/v1/checkouts") {
    checkoutBody = JSON.parse(init.body);
    return reply({ data: { attributes: { url: "https://pay.test/c/abc" } } }, 201);
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

function orderEvent(over = {}, custom = { user_sub: "g-1" }) {
  const body = JSON.stringify({
    meta: { event_name: "order_created", custom_data: custom },
    data: { id: "9001", attributes: { status: "paid", store_id: 11, user_email: "Buyer@Example.com", test_mode: false, first_order_item: { variant_id: 22 }, ...over } },
  });
  return body;
}
const hook = (body, sig) =>
  webhook(new Request("https://trimmel.app/api/webhooks/lemonsqueezy", { method: "POST", body, headers: { "x-signature": sig ?? createHmac("sha256", "whsec").update(body).digest("hex") } }));

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

test("checkout needs a session and sends the account to Lemon Squeezy", async () => {
  assert.equal((await checkout(json("/api/checkout", {}))).status, 401);
  const cookie = await signIn();
  const res = await checkout(json("/api/checkout", {}, { cookie }));
  assert.equal(res.status, 200);
  assert.equal((await res.json()).url, "https://pay.test/c/abc");
  assert.equal(checkoutBody.data.attributes.checkout_data.custom.user_sub, "g-1");
  assert.equal(checkoutBody.data.attributes.checkout_data.email, "buyer@example.com");
  assert.equal(checkoutBody.data.relationships.variant.data.id, "22");
});

test("webhook rejects a bad signature", async () => {
  assert.equal((await hook(orderEvent(), "00")).status, 400);
  assert.equal(kv.size, 0);
});

test("a paid order stores a key that the account page shows, and the key verifies", async () => {
  const cookie = await signIn();
  assert.equal((await hook(orderEvent())).status, 200);
  const body = await (await getMe(cookie)).json();
  assert.ok(body.plus.key.startsWith("TRIM1-"));
  assert.equal(body.plus.orderId, "9001");

  // Same check the Go app makes: Ed25519 over the payload bytes.
  const [p, s] = body.plus.key.slice(6).split(".");
  const payload = Buffer.from(p, "base64url");
  assert.ok(verify(null, payload, createPublicKey(publicKey.export({ format: "pem", type: "spki" })), Buffer.from(s, "base64url")));
  const claims = JSON.parse(payload.toString());
  assert.equal(claims.id, "LS-9001");
  assert.equal(claims.email, "buyer@example.com");
  assert.equal(claims.major, 1);
  assert.equal(claims.devices, 3);
  assert.equal(claims.expires, undefined);
  assert.match(claims.issued, /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/);

  // Already owns Plus: no second checkout.
  assert.equal((await checkout(json("/api/checkout", {}, { cookie }))).status, 409);
});

test("webhook retries do not mint a second key", async () => {
  await hook(orderEvent());
  const first = kv.get("order:9001");
  assert.deepEqual(await (await hook(orderEvent())).json(), { stored: false });
  assert.equal(kv.get("order:9001"), first);
});

test("buying with another email still lands on the signed-in account", async () => {
  const cookie = await signIn("g-1", "me@example.com");
  await hook(orderEvent({ user_email: "paypal-address@example.com" }));
  assert.ok((await (await getMe(cookie)).json()).plus);
});

test("with no account attached, the email still finds the order", async () => {
  await hook(orderEvent({}, {}));
  const cookie = await signIn("g-2", "buyer@example.com");
  assert.ok((await (await getMe(cookie)).json()).plus);
});

test("orders for another store or product, or unpaid, are ignored", async () => {
  for (const over of [{ store_id: 99 }, { first_order_item: { variant_id: 1 } }, { status: "pending" }]) {
    assert.deepEqual(await (await hook(orderEvent(over))).json(), { ignored: true });
  }
  assert.equal(kv.size, 0);
});

test("a refund hides the key and allows a new purchase", async () => {
  const cookie = await signIn();
  await hook(orderEvent());
  const body = JSON.stringify({ meta: { event_name: "order_refunded" }, data: { id: "9001", attributes: {} } });
  assert.deepEqual(await (await hook(body)).json(), { refunded: true });
  const me1 = await (await getMe(cookie)).json();
  assert.equal(me1.plus, null);
  assert.equal(me1.refunded, true);
  assert.equal((await checkout(json("/api/checkout", {}, { cookie }))).status, 200);
});
