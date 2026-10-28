// POST from Stripe. A paid Plus checkout mints a lifetime licence key and stores it against the
// buyer's account; a full refund marks the order so the account page stops showing the key.
// Returns 200 for events it ignores (so Stripe does not retry) and 500 when storing fails (so it does).
import { fail, json } from "../_lib/http.js";
import { PRODUCT_TAG, signatureOK } from "../_lib/stripe.js";
import { mintKey } from "../_lib/licence.js";
import { getOrder, getOrderByPayment, saveOrder, updateOrder } from "../_lib/store.js";

export async function POST(request) {
  const raw = await request.text();
  if (!signatureOK(raw, request.headers.get("stripe-signature"))) return fail(400, "Bad signature.");
  let event;
  try {
    event = JSON.parse(raw);
  } catch {
    return fail(400, "Bad body.");
  }
  const obj = event?.data?.object;
  if (!obj?.id) return json({ ignored: true });
  try {
    switch (event.type) {
      case "checkout.session.completed":
      case "checkout.session.async_payment_succeeded":
        return await paid(obj);
      case "charge.refunded":
        return await refunded(obj);
    }
  } catch (err) {
    console.error("webhook:", event.type, err.message);
    return fail(500, "Could not record the order.");
  }
  return json({ ignored: true });
}

async function paid(s) {
  // Only Plus checkouts created by /api/checkout carry this tag; anything else in the account is ignored.
  if (s.payment_status !== "paid" || s.metadata?.product !== PRODUCT_TAG) return json({ ignored: true });
  const email = String(s.customer_details?.email || s.customer_email || "").toLowerCase();
  if (!email) return fail(400, "Order has no email.");
  const rec = {
    id: s.id,
    email,
    userSub: String(s.metadata?.user_sub || s.client_reference_id || ""),
    paymentIntent: typeof s.payment_intent === "string" ? s.payment_intent : "",
    key: mintKey({ id: s.id, email }),
    issued: new Date().toISOString(),
    testMode: s.livemode === false,
    refunded: false,
  };
  return json({ stored: await saveOrder(rec) });
}

async function refunded(charge) {
  if (!charge.refunded || typeof charge.payment_intent !== "string") return json({ ignored: true }); // partial refunds keep the key
  const rec = await getOrderByPayment(charge.payment_intent);
  if (!rec) return json({ ignored: true });
  rec.refunded = true;
  await updateOrder(rec);
  return json({ refunded: true });
}
