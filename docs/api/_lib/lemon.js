// Lemon Squeezy: create a hosted checkout for the signed-in user, and verify webhook signatures.
import { createHmac, timingSafeEqual } from "node:crypto";
import { siteURL } from "./http.js";

export async function createCheckout({ email, sub }) {
  const { LS_API_KEY, LS_STORE_ID, LS_VARIANT_ID } = process.env;
  if (!LS_API_KEY || !LS_STORE_ID || !LS_VARIANT_ID) throw new Error("Lemon Squeezy settings are missing");
  const res = await fetch("https://api.lemonsqueezy.com/v1/checkouts", {
    method: "POST",
    headers: {
      accept: "application/vnd.api+json",
      "content-type": "application/vnd.api+json",
      authorization: `Bearer ${LS_API_KEY}`,
    },
    body: JSON.stringify({
      data: {
        type: "checkouts",
        attributes: {
          checkout_data: { email, custom: { user_sub: sub } },
          product_options: { redirect_url: `${siteURL()}/account.html?paid=1` },
        },
        relationships: {
          store: { data: { type: "stores", id: String(LS_STORE_ID) } },
          variant: { data: { type: "variants", id: String(LS_VARIANT_ID) } },
        },
      },
    }),
  });
  const body = await res.json().catch(() => ({}));
  const url = body?.data?.attributes?.url;
  if (!res.ok || !url) throw new Error("checkout: " + (body?.errors?.[0]?.detail || res.status));
  return url;
}

// signatureOK compares the X-Signature header (hex HMAC-SHA256 of the raw body) in constant time.
export function signatureOK(rawBody, header) {
  const secret = process.env.LS_WEBHOOK_SECRET;
  if (!secret || !header) return false;
  const want = createHmac("sha256", secret).update(rawBody).digest();
  const got = Buffer.from(String(header), "hex");
  return got.length === want.length && timingSafeEqual(got, want);
}
