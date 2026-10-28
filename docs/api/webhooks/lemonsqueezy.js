// POST from Lemon Squeezy. A paid order mints a lifetime licence key and stores it against the
// buyer's account; a refund marks the order so the account page stops showing the key.
// Returns 200 for events it ignores (so Lemon Squeezy does not retry) and 500 when storing fails
// (so it does).
import { fail, json } from "../_lib/http.js";
import { signatureOK } from "../_lib/lemon.js";
import { mintKey } from "../_lib/licence.js";
import { getOrder, saveOrder, updateOrder } from "../_lib/store.js";

export async function POST(request) {
  const raw = await request.text();
  if (!signatureOK(raw, request.headers.get("x-signature"))) return fail(400, "Bad signature.");
  let event;
  try {
    event = JSON.parse(raw);
  } catch {
    return fail(400, "Bad body.");
  }
  const name = event?.meta?.event_name;
  const order = event?.data;
  if (!order?.id) return json({ ignored: true });
  const a = order.attributes || {};
  try {
    if (name === "order_created") return await created(event, order, a);
    if (name === "order_refunded") return await refunded(order);
  } catch (err) {
    console.error("webhook:", name, err.message);
    return fail(500, "Could not record the order.");
  }
  return json({ ignored: true });
}

async function created(event, order, a) {
  const { LS_STORE_ID, LS_VARIANT_ID } = process.env;
  const variant = a.first_order_item?.variant_id;
  if (a.status !== "paid" || String(a.store_id) !== String(LS_STORE_ID) || String(variant) !== String(LS_VARIANT_ID)) {
    return json({ ignored: true });
  }
  const email = String(a.user_email || "").toLowerCase();
  if (!email) return fail(400, "Order has no email.");
  const id = String(order.id);
  const rec = {
    id,
    email,
    userSub: event.meta?.custom_data?.user_sub ? String(event.meta.custom_data.user_sub) : "",
    key: mintKey({ id: `LS-${id}`, email }),
    issued: new Date().toISOString(),
    testMode: Boolean(a.test_mode),
    refunded: false,
  };
  return json({ stored: await saveOrder(rec) });
}

async function refunded(order) {
  const rec = await getOrder(String(order.id));
  if (!rec) return json({ ignored: true });
  rec.refunded = true;
  await updateOrder(rec);
  return json({ refunded: true });
}
