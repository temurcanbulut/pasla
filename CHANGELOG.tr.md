# Değişiklikler

[English](CHANGELOG.md)

## 0.4.3

- Verify discovery hints with certificate-pinned TLS before changing peer/trust records; ignore unsigned goodbye. Notify on verified renames.
- Reject overflowing and over-16-TiB offers before transfer registration, on desktop and phone paths.
- Consume each phone QR once, throttle guesses per source without revoking another phone's QR; document the active-MITM boundary.
- Scope pairing budgets per source, preserve pending SAS sessions, lock nonce reads and reserve batch folders across concurrent transfers.
- Expand risky Windows extensions and reserved-name handling, with offline regressions.

## 0.4.2

### Güvenlik

- Gelen aktarım mevcut dosyaları korur: benzersiz geçici dosyalar dışlayıcı oluşturulur; aynı Node'un alıcıları tamamlanmış hedefleri ezmez.
- Mevcut geçici/hedef dosyaları, hatalı sağlama özeti ve eşzamanlı aktarımlar için regresyon testleri eklendi.
- Workflow eylemleri ve govulncheck sabitlendi; release derlemesi salt okunur, yazma yetkisi yalnızca ayrı yayımlama işinde.

### Yayın

- v0.4.2-beta.2, değişmez GitHub release dosyaları ve doğrulanabilir yayın attestation kaydı ekler. Uygulama davranışı yamalı beta.1 sürümüyle aynıdır.
- Windows/Linux amd64 ve arm64 için Go 1.27.3 ile yerelde derlenen v0.4.2-beta.1 dosyaları. Önceki beta dosyaları yerine bu güvenlik güncellemesi kullanılmalı.

## 0.4.1

### Güvenlik

- Kaynaktan derleme için Go 1.27.3 veya üzerini zorunlu tut; yayın dosyalarını düzeltilmiş standart kütüphaneyle yeniden derle. crypto/x509 içindeki, güveni kaldırılan bilinen ara sertifikalara ilişkin [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) / CVE-2026-111943 güncellemesini içerir.

### Bakım

- Dependabot #1–#5 güncellemeleri: checkout v7, setup-go v7, configure-pages v6, upload-pages-artifact v5 ve deploy-pages v5.

## 0.4.0

### Yeni
- **Yeni kurumsal kimlik:** Bilgisayar, dosya ve telefonu iki yayla birleştiren logo; lacivert, mavi ve camgöbeği palet. Tüm logo dosyaları vektör ve `assets/brand/generate.py` ile yeniden üretilebilir. Küçük boyutlar için sadeleştirilmiş simge. Ayrıntılar: `docs/BRAND.md`.
- **Windows'ta konsol penceresi yok:** Program çift tıklayınca arka planda açılır, bildirim alanında (saatin yanında) bir simge gösterir. Tıklayınca arayüz açılır, sağ tıklayınca kapatılır. Terminalden çalıştırıldığında çıktı yine o pencereye yazılır.
- Ayarlara **Pasla'yı kapat** düğmesi eklendi.
- **Pencereyi kapatmak Pasla'yı kapatır.** Son Pasla sekmesi kapandıktan birkaç saniye sonra program da kapanır; yenileme sorun olmaz, süren aktarımlar önce biter. Arka planda dosya almaya devam etmek isteyenler için Ayarlar'a **Pencereyi kapatınca da çalışmaya devam etsin** seçeneği eklendi; açıkken Windows'ta bir bildirimle Pasla'nın saatin yanında çalışmaya devam ettiği söylenir.
- Windows'ta kapattıktan sonra bildirim alanında "hayalet" Pasla simgesi kalması düzeltildi.
- Terminal olmadan başlatıldığında kayıtlar ayar klasöründeki `pasla.log` dosyasına yazılıyor; başlayamazsa Windows'ta bir uyarı kutusu çıkıyor.

## 0.3.0

### Yeni
- **İki dil:** Arayüz, telefon sayfası, hata mesajları ve komut satırı İngilizce (varsayılan) ve Türkçe. Bilgisayarda Ayarlar'dan, telefonda EN/TR düğmesinden değiştirilir.
- Başka bir cihazdan gelen hatalar artık bir mesaj anahtarıyla taşınıyor; böylece karşı cihaz hangi dilde olursa olsun, hata sizin seçtiğiniz dilde görünüyor.
- README İngilizce; Türkçesi `README.tr.md` dosyasında.

### Düzeltmeler
- Telefonda, bilgisayardan gelen metinler "gönderildi" olarak görünüyordu.
- Geçersiz IP adresleri (ör. `300.1.1.1`) baştan reddediliyor; ağ hataları teknik ayrıntı yerine anlaşılır bir dille gösteriliyor.

## 0.2.0

**Ad değişikliği:** Proje "Ulak" adını bıraktı ve **Pasla** oldu. Ayar klasörü artık `pasla`; eski `ulak` klasöründeki eşleşmeler taşınmaz, cihazları bir kez yeniden eşleştirin.

### Yeni
- **Telefon bağlantısı:** Uygulama kurmadan, QR kod ve 4 haneli onay numarasıyla telefondan dosya ve metin gönderip alma.
- **Bağımlılıksız QR kodlayıcı** (`internal/qr`).
- **Güvenlik numarası:** Eşleşmiş iki cihazda aynı görünen 12 haneli numara.
- Yeni logo ve simge (SVG); Windows exe'sine gömülü simge, sürüm bilgisi ve manifest.

### Güvenlik
- Yerel arayüz artık oturum anahtarı istiyor. Tarayıcı tek kullanımlık bir kodla oturum açıyor; aynı bilgisayardaki başka kullanıcılar ve programlar arayüzü kullanamıyor.
- Eşleştirme isteklerine katlanarak artan bekleme süresi ve dakikalık sınır getirildi.
- Gelen dosyalara Windows'ta "Web işareti" (Mark of the Web) ekleniyor.
- Çalıştırılabilir dosya içeren gönderimlerde ve eşleşmiş bir cihazın adını taklit eden cihazlarda uyarı gösteriliyor.
- Alıcıda yer yoksa gönderim baştan reddediliyor; bir cihazdan aynı anda en çok 3 bekleyen teklif kabul ediliyor.
- Dosya adı temizleyicisi için fuzz testi eklendi; CI'da `govulncheck` çalışıyor.

## 0.1.0

İlk sürüm: keşif, eşleştirme, şifreli aktarım, metin, komut satırından gönderim.
