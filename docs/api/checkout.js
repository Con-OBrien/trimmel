// POST. Creates a Stripe Checkout Session for the signed-in user and returns its URL.
import { fail, guardPost, json } from "./_lib/http.js";
import { readSession } from "./_lib/session.js";
import { orderFor } from "./_lib/store.js";
import { createCheckout } from "./_lib/stripe.js";

export async function POST(request) {
  const blocked = guardPost(request);
  if (blocked) return blocked;
  const user = readSession(request);
  if (!user) return fail(401, "Sign in first.");
  try {
    const existing = await orderFor(user);
    if (existing && !existing.refunded) return fail(409, "This account already has Trimmel Plus.");
    return json({ url: await createCheckout({ email: user.email, sub: user.sub }) });
  } catch (err) {
    console.error("checkout:", err.message);
    return fail(500, "Checkout is not available right now. Please try again later.");
  }
}
