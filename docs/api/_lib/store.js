// Orders and the accounts they belong to, kept in Upstash Redis (the Vercel KV integration) over its
// REST API, so the site needs no dependencies.
//
//   order:<id>      JSON { id, email, userSub, paymentIntent, key, issued, testMode, refunded }
//                   (id is the Stripe Checkout Session id)
//   pi:<id>         order id, so a refund (which names the payment) finds its order
//   user:<sub>      order id, for the Google account that bought it
//   email:<email>   order id, as a fallback when the buyer used a different account
//
// A user can hold one order. The licence key is stored so the account page can show it again.

async function cmd(...args) {
  // The Vercel integration sets KV_REST_API_*; a database made directly at Upstash gives UPSTASH_REDIS_REST_*.
  const url = process.env.KV_REST_API_URL || process.env.UPSTASH_REDIS_REST_URL;
  const token = process.env.KV_REST_API_TOKEN || process.env.UPSTASH_REDIS_REST_TOKEN;
  if (!url || !token) throw new Error("no Redis REST URL/token: set KV_REST_API_URL and KV_REST_API_TOKEN");
  const res = await fetch(url, {
    method: "POST",
    headers: { authorization: `Bearer ${token}`, "content-type": "application/json" },
    body: JSON.stringify(args),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok || body.error) throw new Error("store: " + (body.error || res.status));
  return body.result;
}

const parse = (s) => {
  try {
    return s ? JSON.parse(s) : null;
  } catch {
    return null;
  }
};

export const getOrder = async (id) => parse(await cmd("GET", `order:${id}`));

export async function getOrderByPayment(paymentIntent) {
  const id = await cmd("GET", `pi:${paymentIntent}`);
  return id ? getOrder(id) : null;
}

// saveOrder writes a new order once. Returns false if that order id already exists (a webhook retry).
export async function saveOrder(order) {
  const set = await cmd("SET", `order:${order.id}`, JSON.stringify(order), "NX");
  if (set !== "OK") return false;
  await cmd("SET", `email:${order.email.toLowerCase()}`, order.id);
  if (order.userSub) await cmd("SET", `user:${order.userSub}`, order.id);
  if (order.paymentIntent) await cmd("SET", `pi:${order.paymentIntent}`, order.id);
  return true;
}

export const updateOrder = (order) => cmd("SET", `order:${order.id}`, JSON.stringify(order));

// orderFor finds the order a signed-in user owns: by account first, then by email.
export async function orderFor({ sub, email }) {
  const id = (await cmd("GET", `user:${sub}`)) || (await cmd("GET", `email:${email.toLowerCase()}`));
  return id ? getOrder(id) : null;
}
