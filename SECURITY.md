# Security policy

[Türkçe](SECURITY.tr.md)

Pasla's goal is that someone who shares your network but whom you do not trust should not be able to read your files, send you something you do not want, or impersonate a device you have paired with. The design is described in [docs/PROTOCOL.md](docs/PROTOCOL.md).

## Supported versions

Security fixes are made only for the latest release. Use v0.4.3-beta.1 or newer for the 2026-11 discovery, transfer-size and phone-session fixes. Older binaries remain historical releases, not supported alternatives.

## Out of scope

- **Someone with administrator access to the computer itself.** They can read the key and session file in the settings directory.
- **Network denial of service (DoS).** Pairing requests and pending offers are limited, but an attacker on the same network can block traffic entirely.
- **Self-chosen device labels.** Raw UDP labels are ignored. Saved metadata comes from a certificate-verified device, but its owner can still choose any display name. Trust remains tied to the fingerprint, not a person or label.
- **Active interception of the phone browser bootstrap.** After a self-signed warning is bypassed, an attacker can substitute the page, read the QR fragment and relay the four-digit approval. That number is not independent authentication against an active LAN MITM. Use the phone feature only on a trusted LAN or use fingerprint-pinned desktop pairing. A fingerprint printed by the same untrusted page does not repair this boundary.

## Reporting a vulnerability

If you find a security vulnerability, please do not open a public issue. Use GitHub's **Private vulnerability reporting**: open the repository's [Security tab](https://github.com/temurcanbulut/pasla/security) and choose **Report a vulnerability** to send a private report. We acknowledge reports within one week and keep the details private until a fix is released.
