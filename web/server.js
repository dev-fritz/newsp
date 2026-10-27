// Production server. It replaces `react-router-serve` for one reason: the BFF
// forwards X-Forwarded-For to the API, which rate limits per client IP, and
// only this process knows the real peer address of each connection. The peer
// is appended to whatever chain the client (or a reverse proxy) sent, so the
// API, which walks the chain from the right and skips trusted proxies, never
// takes a hop the client invented.
import path from "node:path";

import { createRequestHandler } from "@react-router/express";
import compression from "compression";
import express from "express";
import morgan from "morgan";

const BUILD_DIR = path.resolve("build");
const build = await import(path.join(BUILD_DIR, "server", "index.js"));

const app = express();
app.disable("x-powered-by");
// Behind a TLS-terminating proxy the browser sends `Origin: https://…` while
// this process only sees plain HTTP. Trusting the loopback and private ranges
// (the same ones TRUSTED_PROXIES lists in the compose file) lets the proxy's
// X-Forwarded-Proto set the request URL, so React Router's action origin check
// compares like with like. The peer address appended below is unaffected.
app.set("trust proxy", "loopback, linklocal, uniquelocal");
app.use(compression());

// Fingerprinted assets can be cached forever; everything else briefly.
app.use(
  "/assets",
  express.static(path.join(BUILD_DIR, "client", "assets"), { immutable: true, maxAge: "1y" }),
);
app.use(express.static(path.join(BUILD_DIR, "client"), { maxAge: "1h" }));
app.use(morgan("tiny"));

app.use((req, _res, next) => {
  const peer = req.socket.remoteAddress?.replace(/^::ffff:/, "");
  if (peer) {
    const chain = req.headers["x-forwarded-for"];
    req.headers["x-forwarded-for"] = chain ? `${chain}, ${peer}` : peer;
  }
  next();
});

app.use(createRequestHandler({ build, mode: process.env.NODE_ENV }));

const port = Number(process.env.PORT ?? 3000);
const host = process.env.HOST ?? "0.0.0.0";
app.listen(port, host, () => {
  process.stdout.write(`newsp web listening on http://${host}:${port}\n`);
});
