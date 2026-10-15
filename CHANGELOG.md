# Changelog

[Türkçe](CHANGELOG.tr.md)

## 0.4.0

### Added

- **New visual identity:** A logo joining a computer, a file and a phone with two arcs; a navy, blue and cyan palette. All logo files are vectors and can be regenerated with `assets/brand/generate.py`. A simplified icon is provided for small sizes. Details: `docs/BRAND.md`.
- **No console window on Windows:** Double-clicking starts the program in the background and shows an icon in the notification area next to the clock. Click to open the interface; right-click to quit. When launched from a terminal, output still goes to that window.
- Added a **Quit Pasla** button in Settings.
- **Closing the window quits Pasla.** The program exits a few seconds after the last Pasla tab closes; reloading is safe, and running transfers finish first. Settings now includes **Keep running after I close the window** for users who want to keep receiving files in the background. When enabled, a Windows notification says Pasla is still running next to the clock.
- Fixed a "ghost" Pasla icon remaining in the Windows notification area after quitting.
- When launched without a terminal, logs are written to `pasla.log` in the settings directory; a Windows alert appears if startup fails.

## 0.3.0

### Added

- **Two languages:** The interface, phone page, error messages and command line are available in English (the default) and Turkish. Switch in Settings on the computer or with the EN/TR button on the phone.
- Errors from another device now carry a message key, so they appear in your chosen language regardless of the other device's language.
- The README is in English; the Turkish version is in `README.tr.md`.

### Fixed

- Text received from the computer was shown as "sent" on the phone.
- Invalid IP addresses (such as `300.1.1.1`) are rejected up front; network errors are shown in plain language instead of technical details.

## 0.2.0

**Name change:** The project was renamed from "Ulak" to **Pasla**. The settings directory is now `pasla`; pairings in the old `ulak` directory are not migrated, so pair your devices again once.

### Added

- **Phone connection:** Send and receive files and text from a phone without installing an app, using a QR code and a 4-digit confirmation number.
- **Dependency-free QR encoder** (`internal/qr`).
- **Safety number:** A 12-digit number that is the same on both paired devices.
- New logo and icon (SVG); embedded icon, version information and manifest in the Windows executable.

### Security

- The local interface now requires a session key. The browser signs in with a one-time code; other users and programs on the same computer cannot use the interface.
- Added exponential backoff and a per-minute limit for pairing requests.
- Received files get the Mark of the Web on Windows.
- Warnings are shown for transfers containing executables and for devices impersonating the name of a paired device.
- Transfers are refused up front if the receiver has insufficient space; at most 3 pending offers from one device are accepted at a time.
- Added a fuzz test for the filename sanitiser; CI runs `govulncheck`.

## 0.1.0

Initial release: discovery, pairing, encrypted transfer, text and command-line sending.
