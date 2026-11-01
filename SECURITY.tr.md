# Güvenlik politikası

[English](SECURITY.md)

Pasla'nın amacı şudur: aynı ağı paylaştığınız ama güvenmediğiniz biri dosyalarınızı okuyamamalı, size istemediğiniz bir şey gönderememeli ve eşleştiğiniz bir cihazın kılığına girememeli. Tasarımın ayrıntıları [docs/PROTOCOL.tr.md](docs/PROTOCOL.tr.md) belgesinde.

## Desteklenen sürümler

Güvenlik düzeltmeleri yalnızca en son sürüme yapılır.

## Kapsam dışı

- **Bilgisayarın kendisinde yönetici yetkisi olan biri.** Ayar klasöründeki anahtarı ve oturum dosyasını okuyabilir.
- **Ağ üzerinden hizmeti engelleme (DoS).** Eşleştirme istekleri ve bekleyen teklifler sınırlıdır, ama aynı ağdaki bir saldırgan trafiği tamamen engelleyebilir.
- **Cihaz sahibinin seçtiği etiketler.** Ham UDP etiketleri yoksayılır. Kaydedilen metaveri sertifikası doğrulanan cihazdan gelir; sahibi yine de istediği görünen adı seçebilir. Güven kişiye ya da isme değil, parmak izine bağlıdır.
- **Telefon tarayıcısının başlangıcına aktif müdahale.** Kendinden imzalı sertifika uyarısı geçildikten sonra saldırgan sayfayı değiştirebilir, QR fragmentini okuyabilir ve dört haneli onayı aktarabilir. Bu numara aktif LAN MITM'ine karşı bağımsız doğrulama değildir. Telefon özelliğini güvenilir LAN'da kullanın veya parmak izi sabitlenen masaüstü eşleşmesini seçin. Aynı güvensiz sayfada yazan kısa parmak izi bu sınırı düzeltmez.

## Açık bildirme

Bir güvenlik açığı bulduysanız lütfen herkese açık bir issue açmayın. GitHub'ın **Private vulnerability reporting** özelliğiyle, deponun [Security sekmesindeki](https://github.com/temurcanbulut/pasla/security) **Report a vulnerability** düğmesinden gizli bir bildirim gönderin. Bildirimi aldığımızı bir hafta içinde doğrularız ve düzeltme yayımlanana kadar ayrıntıları gizli tutarız.

2026-11 keşif, boyut sınırı ve telefon oturumu düzeltmeleri için v0.4.3-beta.1 veya yenisini kullanın. Önceki ikililer tarihsel kayıttır; desteklenen alternatif değildir.
