// GET. Who is signed in, and the Plus licence they own, if any. Signed out, it returns the public
// Google client ID the page needs to show the sign-in button.
import { fail, json } from "./_lib/http.js";
import { readSession } from "./_lib/session.js";
import { orderFor } from "./_lib/store.js";

export async function GET(request) {
  const user = readSession(request);
  if (!user) return json({ signedIn: false, googleClientId: process.env.GOOGLE_WEB_CLIENT_ID || "" });
  let order;
  try {
    order = await orderFor(user);
  } catch (err) {
    console.error("me:", err.message);
    return fail(500, "Your account could not be loaded. Please try again.");
  }
  const plus = order && !order.refunded ? { key: order.key, orderId: order.id, issued: order.issued } : null;
  return json({ signedIn: true, email: user.email, plus, refunded: Boolean(order?.refunded) });
}
