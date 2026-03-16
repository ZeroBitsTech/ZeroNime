import crypto from "k6/crypto";
import encoding from "k6/encoding";
import http from "k6/http";
import { check, sleep } from "k6";

const baseUrl = __ENV.BASE_URL || "http://127.0.0.1:8080";
const deviceId = __ENV.DEVICE_ID;
const secret = __ENV.SECRET;

if (!deviceId || !secret) {
  throw new Error("DEVICE_ID and SECRET env are requird");
}

export const options = {
  vus: Number(__ENV.VUS || 5),
  duration: __ENV.DURATION || "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<800", "p(99)<1500"],
  },
};

function sign(method, path) {
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const nonce = encoding.b64encode(crypto.randomBytes(12), "rawstd").replace(/[^a-zA-Z0-9]/g, "").slice(0, 16);
  const payload = `${method}\n${path}\n${timestamp}\n${deviceId}\n${nonce}`;
  const signature = encoding.hexEncode(crypto.hmac("sha256", secret, payload, "binary"));
  return {
    "X-Device-ID": deviceId,
    "X-Timestamp": timestamp,
    "X-Nonce": nonce,
    "X-Signature": signature,
  };
}

function get(path) {
  const res = http.get(`${baseUrl}${path}`, {
    headers: sign("GET", path),
  });
  check(res, {
    [`${path} status 200`]: (r) => r.status === 200,
  });
  return res;
}

export default function () {
  get("/v1/home?page=1");
  get("/v1/search?keyword=Naruto");
  get("/v1/list");
  sleep(1);
}
