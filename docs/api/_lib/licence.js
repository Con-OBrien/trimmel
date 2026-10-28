// Mints Trimmel Plus keys in the format internal/license expects:
//   TRIM1-<base64url(JSON claims)>.<base64url(Ed25519 signature over those bytes)>
// LICENCE_PRIVATE_KEY is the base64 file written by `licensegen keygen` (a Go ed25519 private key:
// 32-byte seed followed by the 32-byte public key). Only the seed is needed here.
import { createPrivateKey, sign } from "node:crypto";

const PKCS8_ED25519_PREFIX = Buffer.from("302e020100300506032b657004220420", "hex");

function privateKey() {
  const raw = Buffer.from((process.env.LICENCE_PRIVATE_KEY || "").trim(), "base64");
  if (raw.length !== 64) throw new Error("LICENCE_PRIVATE_KEY is not a licensegen private key");
  return createPrivateKey({ key: Buffer.concat([PKCS8_ED25519_PREFIX, raw.subarray(0, 32)]), format: "der", type: "pkcs8" });
}

// A lifetime key: no expiry, three PCs, valid for app versions up to LICENCE_MAJOR (default 1).
// account is the buyer's Google account id; the app only activates the key for that account.
export function mintKey({ id, email, account = "", now = new Date() }) {
  const major = Number.parseInt(process.env.LICENCE_MAJOR || "1", 10);
  const claims = {
    id: String(id),
    email,
    issued: now.toISOString().replace(/\.\d+Z$/, "Z"),
    major: Number.isInteger(major) && major >= 0 ? major : 1,
    devices: 3,
    ...(account ? { account } : {}),
  };
  const payload = Buffer.from(JSON.stringify(claims));
  const sig = sign(null, payload, privateKey());
  return `TRIM1-${payload.toString("base64url")}.${sig.toString("base64url")}`;
}
