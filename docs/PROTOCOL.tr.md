# Pasla protokolü (v1)

[English](PROTOCOL.md)

Bu belge Pasla cihazlarının birbiriyle ve telefonlarla nasıl konuştuğunu anlatır. Amaç, kodu okumadan tasarımı denetleyebilmek ve başka bir istemci yazabilmektir.

## Kimlik

Her cihaz ilk açılışta bir ECDSA P-256 anahtar çifti ve 30 yıl geçerli, kendinden imzalı bir X.509 sertifikası üretir (`identity.crt`, `identity.key`; anahtar dosyası yalnızca sahibince okunabilir). Cihazın kimliği, sertifikanın DER kodlamasının SHA-256 özetidir: 64 onaltılık karakterden oluşan **parmak izi**. Güven her zaman parmak izine verilir; ad ve IP adresi yalnızca bilgi amaçlıdır.

## Keşif (UDP 47820)

Her cihaz üç saniyede bir, her yerel IPv4 arabiriminin yayın adresine ve `255.255.255.255`'e şu JSON'u yollar:

```json
{"app":"pasla","v":1,"fp":"<parmak izi>","name":"Salon PC","os":"windows","port":47821}
```

Yeni bir cihaz duyan, göndericiye bir kerelik `"reply":true` işaretli bir yanıt yollar; böylece yeni açılan cihaz beklemeden listelenir. Kapanan cihaz `"bye":true` yollar. 12 saniye sesi çıkmayan cihaz listeden düşer. Eşleşmiş ama yayını duyulmayan cihazlar, son bilinen adreslerinde beş saniyede bir doğrudan sorulur.

Bu paketler imzasızdır ve sır içermez. İçerdikleri her iddia sonradan TLS ile doğrulanır. Eşleşmiş bir cihazın adını taşıyan ama parmak izi farklı bir cihaz arayüzde "ad taklidi olabilir" uyarısıyla gösterilir.

## Taşıma (TCP 47821)

HTTPS, TLS 1.3, karşılıklı sertifika. Sunucu her istemciden sertifika ister (`RequireAnyClientCert`) ve istemcinin parmak izini kimlik olarak kullanır. İstemci, sunucu sertifikasını bir sertifika otoritesi zinciriyle değil, beklediği parmak iziyle karşılaştırarak doğrular.

| Uç | Kim çağırabilir | Ne yapar |
|---|---|---|
| `GET /v1/info` | herkes | ad, OS, port, parmak izi |
| `POST /v1/pair/start` | herkes (hız sınırlı) | eşleştirme oturumu açar |
| `POST /v1/pair/nonce` | oturum sahibi | nonce takası; kod oluşur |
| `POST /v1/pair/finish` | oturum sahibi | karşı tarafın kararını bekler |
| `POST /v1/pair/cancel` | oturum sahibi | oturumu kapatır |
| `POST /v1/unpair` | eşleşmiş | "beni unut" |
| `POST /v1/offer` | eşleşmiş | dosya listesi teklif eder, kabulü bekler |
| `PUT /v1/transfer/{id}/{i}` | eşleşmiş | i'nci dosyanın baytları |
| `POST /v1/transfer/{id}/cancel` | eşleşmiş | aktarımı iptal eder |
| `POST /v1/message` | eşleşmiş | metin (en çok 1 MB) |

Hatalar `{"error": "...", "key": "...", "args": [...]}` gövdesiyle döner. `error` İngilizce metindir. `key` ve `args`, karşı cihazın mesajı kendi kullanıcısının dilinde gösterebilmesi içindir (anahtar, İngilizce biçim metninin kendisidir; argümanlar düz metindir). İstemciler isterse `X-Pasla-Lang: tr` başlığıyla doğrudan Türkçe hata alabilir.

## Eşleştirme

A başlatır, B yanıtlar. TLS sayesinde ikisi de karşısındakinin parmak izini bilir (`fpA`, `fpB`).

1. A → B `start {name, os, port}`. B, 32 baytlık rastgele bir `Nb` seçer ve `commit = SHA256(Nb)` döner.
2. A, 32 baytlık rastgele bir `Na` seçer: A → B `nonce {sid, Na}`. B, `Nb`'yi açıklar.
3. A, `SHA256(Nb) == commit` olduğunu doğrular.
4. İki taraf da `kod = uint32_be(SHA256("pasla-pair-v1|" + fpA + "|" + fpB + "|" + Na + Nb)[0:4]) mod 10^6` hesaplar ve 6 hane olarak gösterir.
5. Kullanıcı iki ekrandaki kodu karşılaştırır ve iki tarafta da onaylar. A, kendi onayından sonra `finish` çağırır; çağrı, B'deki kullanıcı karar verene dek bekler. İki taraf da onaylarsa her biri ötekinin parmak izini güvenilenler listesine yazar.

**Neden güvenli:** Araya giren M, A ile ve B ile ayrı iki TLS oturumu tutmak zorundadır ve bu oturumlardaki parmak izi çiftleri farklıdır (`fpA|fpM` ve `fpM|fpB`). M, A'ya karşı yanıtlayan taraf olduğu için `Nm`'yi `Na`'yı görmeden taahhüt etmek zorundadır; B'ye karşı başlatan taraf olduğu için de `Nb`'yi yalnızca taahhüt olarak görür. İki kodu eşitlemek için elinde tek bir rastgele deneme kalır: başarı olasılığı 10⁻⁶.

**Hız sınırı:** Bir eşleştirme isteği reddedilir, iptal edilir ya da yanıtsız kalırsa, isteği yapan parmak izi **ve** IP adresi önce 30 saniye, sonraki her başarısızlıkta iki katı süre (en çok 15 dakika) bekletilir. Parmak izi üretmek bedava olduğu için IP de sayılır. Cihaz, toplamda dakikada en fazla 6 yeni istek kabul eder ve aynı anda en fazla 4 isteği bekletir. Oturumlar 3 dakikada düşer.

**Güvenlik numarası:** Eşleşmiş iki cihaz `SHA256("pasla-safety-v1|" + min(fp) + "|" + max(fp))` değerinden 12 haneli bir numara türetir. Numara iki tarafta da aynıdır ve istendiği zaman karşılaştırılabilir.

## Aktarım

1. Gönderen `offer {id, files: [{name, size}]}` yollar. `id` 128 bit rastgeledir (32 onaltılık karakter); `name`, `/` ile ayrılmış göreli yoldur.
2. Alıcı:
   - Bekleyen teklif sayısını denetler (bir cihazdan en çok 3).
   - Her yolu temizler: `..`, mutlak yollar, kontrol ve biçim karakterleri, Windows'a yasak karakter ve adlar.
   - Boş alanı denetler: toplam boyut ile 256 MB pay sığmıyorsa teklifi gerekçesiyle reddeder.
   - Çalıştırılabilir dosya türlerini sayar ve arayüzde uyarı gösterir.
   - Kullanıcıya sorar ya da otomatik kabul eder ve `{accepted: true|false}` döner.

   Kabul edilince her dosyanın hedefi belirlenir. Üst düzeydeki ad zaten varsa bütün öğe ` (2)`, ` (3)`… ekiyle yeniden adlandırılır.
3. Gönderen her dosyayı ayrı bir `PUT` ile yollar; aynı anda en çok üç dosya gider. Gövde parçalı (chunked) akar. Dosyanın SHA-256 özeti `X-Pasla-Sha256` HTTP trailer'ında gelir. Alıcı `<hedef>.paslapart` dosyasına yazar, boyutu ve özeti doğrular, sonra dosyayı yeniden adlandırır. Uyuşmazlıkta yarım dosya silinir. Windows'ta tamamlanan dosyaya `Zone.Identifier` akışı (`ZoneId=3`) eklenir.
4. Her iki taraf da `cancel` ile aktarımı durdurabilir; bu çağrıdan sonra gelen `PUT` istekleri `410 Gone` alır.

## Telefon bağlantısı (TCP 47823, HTTPS)

Telefon bir uygulama kurmaz; bilgisayarın sunduğu sayfayı tarayıcıda açar.

1. Bilgisayarda kullanıcı **Telefonu bağla**'ya bastığında port açılır ve 128 bitlik rastgele bir anahtar üretilir. QR kod `https://<ip>:47823/#k=<anahtar>` adresini taşır. Anahtar adresin `#` sonrasındadır; tarayıcılar bu kısmı ağ üzerinden göndermez, sunucu kayıtlarına da düşmez.
2. Sayfa anahtarı bir kez `POST /p/login` ile yollar. Anahtar 10 dakika geçerlidir ve ilk başarılı girişte tüketilir. Kaynak IP başına dakikada 8 deneme sınırı vardır; yanlış denemeler başka telefonun QR kodunu iptal etmez. Doğruysa telefona `HttpOnly; Secure; SameSite=Strict` bir oturum çerezi ve 4 haneli bir kontrol numarası verilir.
3. Bilgisayar "X bağlanmak istiyor, numara 1234" der. Kullanıcı, telefondaki numara aynıysa izin verir. O ana kadar telefon yalnızca kendi durumunu okuyabilir.
4. Onaylı telefon `/p/api/offer` + `/p/api/upload/{id}/{i}` ile bilgisayara dosya gönderir, `/p/api/outbox/{id}` ile bilgisayarın hazırladıklarını indirir, `/p/api/message` ile metin yollar.

Sayfa, cihaz sertifikasıyla TLS üzerinden sunulur; trafik şifrelidir ama sertifika kendinden imzalı olduğu için telefon tarayıcısı bir kez uyarı gösterir. Durum değiştiren isteklerde `Origin` denetlenir. 30 dakika kullanılmayan telefonun oturumu kapanır; hiç telefon kalmayıp anahtar da süresini doldurunca port kendiliğinden kapanır.

**Sınır:** Tarayıcılar HTTP trailer gönderemediği için telefondan gelen dosyalarda ayrı bir SHA-256 kontrolü yoktur; bütünlüğü TLS sağlar.

## Yerel arayüz (127.0.0.1:47822)

Arayüz aynı programın içinden sunulur. JSON API'si `/api/*` altındadır; canlı güncellemeler `/api/events` üzerinden Server-Sent Events ile gelir.

- **Oturum:** Program her açılışta rastgele bir oturum anahtarı üretir ve ayar klasöründeki `ui.token` dosyasına (izinler `0600`) yazar. API, bu anahtarı `X-Pasla-Token` başlığında ya da `pasla_session` çerezinde ister. Tarayıcı anahtarı hiçbir zaman adres çubuğunda görmez: program, tarayıcıyı `/open?c=<kod>` ile açar. Bu kod tek kullanımlıktır, 2 dakikada geçersiz olur ve `HttpOnly; SameSite=Strict` bir çerezle değiştirilir. Böylece aynı bilgisayardaki başka bir kullanıcı ya da program, `127.0.0.1`'e erişebilse bile arayüzü kullanamaz.
- **Tarayıcı saldırıları:** `Host` başlığı bir geri döngü adı olmalıdır (DNS yeniden bağlama saldırılarına karşı). Durum değiştiren isteklerde `Origin` yoksa ya da arayüzün kendisiyse kabul edilir (siteler arası isteklere karşı). Bunlara ek olarak sıkı bir CSP, `X-Frame-Options: DENY` ve `nosniff` uygulanır.

## İnceleme sonrası — Kasım 2026

UDP keşfi yalnızca doğrulanmamış adres ipucudur. İsim, adres ve son-görülme zamanı ancak iddia edilen parmak izine sabitlenmiş TLS bağlantısından eşleşen `/v1/info` geldikten sonra kayda girer. İmzasız veda paketi yoksayılır; sessiz cihaz zaman aşımıyla görünmez olur. Aynı anda dört probe ve kısa ömürlü 256 adres kovası sınırı vardır. Doğrulanmış eşleşmiş cihaz adı değişince UI bildirim verir. LAN taşırması erişilebilirliği yine etkileyebilir.

Telefon tarayıcısında kendinden imzalı sertifika uyarısını geçmek, aktif LAN MITM'ine karşı sayfayı doğrulamaz. Sahte sayfa QR fragmentini okuyup dört haneli numarayı aktarabilir. Numara yanlışlıkla/QR fotoğrafıyla bağlantıyı azaltır; aktif MITM koruması değildir. Telefon özelliğini güvenilir ağda kullanın veya parmak izi sabitlenen masaüstü eşleşmesini seçin. Aynı sayfanın gösterdiği kısa parmak izi bağımsız doğrulama sayılmaz.

Dosya ve toplam boyutu kayıt oluşturulmadan önce denetlenir; sınır 16 TiB'dır. Boş alan rezervi korunur. Eşzamanlı alımlar üst klasör adlarını kilit altında rezerve eder; dosya oluşturma ayrıca mevcut dosyaların üzerine yazmayı engeller.
