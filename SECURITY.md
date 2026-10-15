# Security policy

[Türkçe](SECURITY.tr.md)

Pasla's goal is that someone who shares your network but whom you do not trust should not be able to read your files, send you something you do not want, or impersonate a device you have paired with. The design is described in [docs/PROTOCOL.md](docs/PROTOCOL.md).

## Supported versions

Security fixes are made only for the latest release.

## Out of scope

- **Someone with administrator access to the computer itself.** They can read the key and session file in the settings directory.
- **Network denial of service (DoS).** Pairing requests and pending offers are limited, but an attacker on the same network can block traffic entirely.
- **False names and operating systems in discovery packets.** This information is not verified; trust is based only on the fingerprint. Impersonated names are flagged in the interface.
- **The phone browser's certificate warning.** Users bypass the warning themselves when connecting a phone. Even if an attacker intercepts this step, they cannot perform any action without the 4-digit approval on the computer. This path is nevertheless not as strong as a connection between two paired computers.

## Reporting a vulnerability

If you find a security vulnerability, please do not open a public issue. Use GitHub's **Private vulnerability reporting**: open the repository's [Security tab](https://github.com/temurcanbulut/pasla/security) and choose **Report a vulnerability** to send a private report. We acknowledge reports within one week and keep the details private until a fix is released.
