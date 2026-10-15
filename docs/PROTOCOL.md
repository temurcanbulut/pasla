# Pasla protocol (v1)

[Türkçe](PROTOCOL.tr.md)

This document describes how Pasla devices communicate with each other and with phones. Its purpose is to make the design auditable without reading the code and to enable other clients to be written.

## Identity

On first launch, each device generates an ECDSA P-256 key pair and a self-signed X.509 certificate valid for 30 years (`identity.crt`, `identity.key`; the key file is readable only by its owner). The device's identity is the SHA-256 digest of the certificate's DER encoding: a **fingerprint** of 64 hexadecimal characters. Trust is always granted to the fingerprint; names and IP addresses are informational only.

## Discovery (UDP 47820)

Every three seconds, each device sends the following JSON to the broadcast address of each local IPv4 interface and to `255.255.255.255`:

```json
{"app":"pasla","v":1,"fp":"<fingerprint>","name":"Living room PC","os":"windows","port":47821}
```

A device that hears a new device sends it a one-time response marked `"reply":true`, so a newly started device appears immediately. A device that is shutting down sends `"bye":true`. Devices not heard from for 12 seconds disappear from the list. Paired devices whose broadcasts are not heard are queried directly at their last known addresses every five seconds.

These packets are unsigned and contain no secrets. Each claim they carry is verified later over TLS. A device with the name of a paired device but a different fingerprint is shown with a "possible impersonation" warning in the interface.

## Transport (TCP 47821)

HTTPS, TLS 1.3, mutual certificates. The server requires a certificate from every client (`RequireAnyClientCert`) and uses the client's fingerprint as its identity. The client verifies the server certificate against the expected fingerprint rather than a certificate authority chain.

| Endpoint | Who can call it | What it does |
|---|---|---|
| `GET /v1/info` | anyone | name, OS, port, fingerprint |
| `POST /v1/pair/start` | anyone (rate-limited) | opens a pairing session |
| `POST /v1/pair/nonce` | session owner | exchanges nonces; produces the code |
| `POST /v1/pair/finish` | session owner | waits for the other side's decision |
| `POST /v1/pair/cancel` | session owner | closes the session |
| `POST /v1/unpair` | paired | "forget me" |
| `POST /v1/offer` | paired | offers a file list and waits for acceptance |
| `PUT /v1/transfer/{id}/{i}` | paired | bytes of the i-th file |
| `POST /v1/transfer/{id}/cancel` | paired | cancels the transfer |
| `POST /v1/message` | paired | text (at most 1 MB) |

Errors return a `{"error": "...", "key": "...", "args": [...]}` body. `error` is English text. `key` and `args` let the other device display the message in its user's language (the key is the English format string itself; arguments are plain text). Clients can also request Turkish errors directly with the `X-Pasla-Lang: tr` header.

## Pairing

A initiates and B responds. TLS gives both sides the other's fingerprint (`fpA`, `fpB`).

1. A → B `start {name, os, port}`. B chooses a random 32-byte `Nb` and returns `commit = SHA256(Nb)`.
2. A chooses a random 32-byte `Na`: A → B `nonce {sid, Na}`. B reveals `Nb`.
3. A verifies that `SHA256(Nb) == commit`.
4. Both sides calculate `code = uint32_be(SHA256("pasla-pair-v1|" + fpA + "|" + fpB + "|" + Na + Nb)[0:4]) mod 10^6` and display it as 6 digits.
5. The user compares the codes on the two screens and confirms on both sides. After its own confirmation, A calls `finish`; this call waits until the user at B decides. If both sides approve, each writes the other's fingerprint to its trusted list.

**Why this is secure:** An intermediary M must maintain two separate TLS sessions, one with A and one with B, and the fingerprint pairs in those sessions differ (`fpA|fpM` and `fpM|fpB`). Because M responds to A, it must commit to `Nm` before seeing `Na`; because it initiates to B, it sees `Nb` only as a commitment. It has just one random attempt to make the two codes match: the success probability is 10⁻⁶.

**Rate limit:** If a pairing request is declined, cancelled or unanswered, the requesting fingerprint **and** IP address must wait 30 seconds, doubling with each subsequent failure up to 15 minutes. The IP address is counted too because generating fingerprints costs nothing. A device accepts at most 6 new requests per minute in total and keeps at most 4 pending at once. Sessions expire after 3 minutes.

**Safety number:** Two paired devices derive a 12-digit number from `SHA256("pasla-safety-v1|" + min(fp) + "|" + max(fp))`. The number is the same on both sides and can be compared at any time.

## Transfer

1. The sender sends `offer {id, files: [{name, size}]}`. `id` is a random 128-bit value (32 hexadecimal characters); `name` is a relative path separated by `/`.
2. The receiver:
   - Checks the number of pending offers (at most 3 from one device).
   - Sanitises each path: `..`, absolute paths, control and format characters, and characters and names forbidden on Windows.
   - Checks free space: if the total size plus a 256 MB margin will not fit, it rejects the offer with a reason.
   - Counts executable file types and shows a warning in the interface.
   - Asks the user or accepts automatically and returns `{accepted: true|false}`.

   Once accepted, a destination is chosen for each file. If the top-level name already exists, the whole item is renamed with a ` (2)`, ` (3)`… suffix.
3. The sender sends each file in a separate `PUT`, with at most three files in flight at once. The body is streamed with chunked encoding. The file's SHA-256 digest arrives in the `X-Pasla-Sha256` HTTP trailer. The receiver writes to `<destination>.paslapart`, verifies the size and digest, and then renames the file. On a mismatch, the partial file is deleted. On Windows, a `Zone.Identifier` stream (`ZoneId=3`) is added to the completed file.
4. Either side can stop a transfer with `cancel`; subsequent `PUT` requests receive `410 Gone`.

## Phone connection (TCP 47823, HTTPS)

The phone installs no app; it opens the page served by the computer in its browser.

1. When the user clicks **Connect a phone** on the computer, the port opens and a random 128-bit key is generated. The QR code contains `https://<ip>:47823/#k=<key>`. The key is after `#`; browsers do not send this part over the network, so it does not appear in server logs.
2. The page sends the key once with `POST /p/login`. The key is valid for 10 minutes and is revoked after 8 incorrect attempts. If it is correct, the phone receives an `HttpOnly; Secure; SameSite=Strict` session cookie and a 4-digit check number.
3. The computer says "X wants to connect, number 1234". The user approves if the number on the phone matches. Until then, the phone can read only its own status.
4. An approved phone sends files to the computer with `/p/api/offer` + `/p/api/upload/{id}/{i}`, downloads items prepared by the computer with `/p/api/outbox/{id}`, and sends text with `/p/api/message`.

The page is served over TLS using the device certificate; traffic is encrypted, but the phone browser shows a warning once because the certificate is self-signed. Requests that change state have their `Origin` checked. A phone session closes after 30 minutes without use; when no phones remain and the key has expired, the port closes automatically.

**Limitation:** Because browsers cannot send HTTP trailers, files from phones have no separate SHA-256 check; TLS provides integrity.

## Local interface (127.0.0.1:47822)

The interface is served by the same program. Its JSON API is under `/api/*`; live updates arrive through Server-Sent Events at `/api/events`.

- **Session:** On every launch, the program generates a random session key and writes it to `ui.token` in the settings directory (permissions `0600`). The API requires this key in the `X-Pasla-Token` header or the `pasla_session` cookie. The browser never sees the key in its address bar: the program opens the browser at `/open?c=<code>`. This code is single-use, expires after 2 minutes and is exchanged for an `HttpOnly; SameSite=Strict` cookie. Thus another user or program on the same computer cannot use the interface even if it can reach `127.0.0.1`.
- **Browser attacks:** The `Host` header must be a loopback name (against DNS rebinding). Requests that change state are accepted if `Origin` is absent or matches the interface itself (against cross-site requests). A strict CSP, `X-Frame-Options: DENY` and `nosniff` are also applied.
