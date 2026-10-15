# Contributing

[Türkçe](CONTRIBUTING.tr.md)

Contributions are welcome. A few principles:

- **No external dependencies.** Pasla must run as a single file on networks without internet access. Open an issue to discuss adding a library first.
- **The interface needs no build step.** The HTML, CSS and JavaScript under `internal/ui/static` are embedded directly. No external fonts, scripts or CDNs are used.
- **Every change that affects security** must include tests and update [docs/PROTOCOL.md](docs/PROTOCOL.md).
- **Interface text is in Turkish.** It explains what happens in plain language, and buttons describe what they will do ("Send", "Accept").

Before submitting:

```sh
make vet test
```
