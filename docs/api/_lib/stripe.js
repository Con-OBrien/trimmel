// Stripe Managed Payments (Stripe is the merchant of record): create a hosted Checkout Session for
// the signed-in user, and verify webhook signatures. Plain fetch, no SDK.
import { createHmac, timingSafeEqual } from "node:crypto";
import { siteURL } from "./http.js";

// managed_payments needs this API version or later.
const API_VERSION = "2026-04-22.dahlia";
export const PRODUCT_TAG = "trimmel-plus";

export async function createCheckout({ email, sub }) {
  const { STRIPE_SECRET_KEY, STRIPE_PRICE_ID } = process.env;
  if (!STRIPE_SECRET_KEY || !STRIPE_PRICE_ID) throw new Error("Stripe settings are missing");
  const form = new URLSearchParams({
    mode: "payment",
    "line_items[0][price]": STRIPE_PRICE_ID,
    "line_items[0][quantity]": "1",
    "managed_payments[enabled]": "true",
    customer_email: email,
    client_reference_id: sub,
    "metadata[user_sub]": sub,
    "metadata[product]": PRODUCT_TAG,
    success_url: `${siteURL()}/account.html?paid=1`,
    cancel_url: `${siteURL()}/account.html`,
  });
  const res = await fetch("https://api.stripe.com/v1/checkout/sessions", {
    method: "POST",
    headers: {
      authorization: `Bearer ${STRIPE_SECRET_KEY}`,
      "stripe-version": API_VERSION,
      "content-type": "application/x-www-form-urlencoded",
    },
    body: form,
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok || !body.url) throw new Error("checkout: " + (body?.error?.message || res.status));
  return body.url;
}

// signatureOK checks the Stripe-Signature header: "t=<unix>,v1=<hex>[,v1=<hex>]", where v1 is the
// HMAC-SHA256 of "<t>.<raw body>" with the endpoint's signing secret. Timestamps older than five
// minutes are refused, which stops replays.
export function signatureOK(rawBody, header, now = Date.now()) {
  const secret = process.env.STRIPE_WEBHOOK_SECRET;
  if (!secret || !header) return false;
  let t = "";
  const sigs = [];
  for (const part of String(header).split(",")) {
    const i = part.indexOf("=");
    if (i < 0) continue;
    const k = part.slice(0, i).trim();
    const v = part.slice(i + 1).trim();
    if (k === "t") t = v;
    else if (k === "v1") sigs.push(v);
  }
  if (!/^\d+$/.test(t) || Math.abs(now / 1000 - Number(t)) > 300) return false;
  const want = createHmac("sha256", secret).update(`${t}.${rawBody}`).digest();
  return sigs.some((s) => {
    const got = Buffer.from(s, "hex");
    return got.length === want.length && timingSafeEqual(got, want);
  });
}
