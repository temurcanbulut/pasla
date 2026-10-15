# Güvenlik politikası

[English](SECURITY.md)

Pasla'nın amacı şudur: aynı ağı paylaştığınız ama güvenmediğiniz biri dosyalarınızı okuyamamalı, size istemediğiniz bir şey gönderememeli ve eşleştiğiniz bir cihazın kılığına girememeli. Tasarımın ayrıntıları [docs/PROTOCOL.tr.md](docs/PROTOCOL.tr.md) belgesinde.

## Desteklenen sürümler

Güvenlik düzeltmeleri yalnızca en son sürüme yapılır.

## Kapsam dışı

- **Bilgisayarın kendisinde yönetici yetkisi olan biri.** Ayar klasöründeki anahtarı ve oturum dosyasını okuyabilir.
- **Ağ üzerinden hizmeti engelleme (DoS).** Eşleştirme istekleri ve bekleyen teklifler sınırlıdır, ama aynı ağdaki bir saldırgan trafiği tamamen engelleyebilir.
- **Keşif paketlerindeki adın ve işletim sisteminin sahte olması.** Bu bilgiler doğrulanmaz; güven yalnızca parmak izine dayanır. Taklit eden adlar arayüzde işaretlenir.
- **Telefon tarayıcısının sertifika uyarısı.** Telefon bağlantısında kullanıcı uyarıyı kendisi geçer. Bu adımda saldırgan araya girerse bile, bilgisayardaki 4 haneli onay olmadan hiçbir işlem yapamaz. Yine de bu yol, eşleştirilmiş iki bilgisayar arasındaki bağlantı kadar güçlü değildir.

## Açık bildirme

Bir güvenlik açığı bulduysanız lütfen herkese açık bir issue açmayın. GitHub'ın **Private vulnerability reporting** özelliğiyle, deponun [Security sekmesindeki](https://github.com/temurcanbulut/pasla/security) **Report a vulnerability** düğmesinden gizli bir bildirim gönderin. Bildirimi aldığımızı bir hafta içinde doğrularız ve düzeltme yayımlanana kadar ayrıntıları gizli tutarız.
