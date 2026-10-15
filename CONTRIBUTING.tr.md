# Katkı

[English](CONTRIBUTING.md)

Katkılarınızı bekliyoruz. Birkaç ilke:

- **Dış bağımlılık yok.** Pasla internete kapalı ağlarda tek dosya olarak çalışmalı. Bir kütüphane eklemeden önce issue açıp konuşalım.
- **Arayüz derleme adımı gerektirmez.** `internal/ui/static` altındaki HTML, CSS ve JavaScript doğrudan gömülür. Dışarıdan font, betik ya da CDN kullanılmaz.
- **Güvenliği etkileyen her değişiklik** testle birlikte gelmeli ve [docs/PROTOCOL.tr.md](docs/PROTOCOL.tr.md) belgesini de güncellemeli.
- **Arayüz metinleri Türkçe.** Ne olduğunu sade bir dille söyler, düğmeler ne yapacaklarını anlatır ("Gönder", "Kabul et").

Göndermeden önce:

```sh
make vet test
```
