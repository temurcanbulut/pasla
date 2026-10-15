# Değişiklikler

[English](CHANGELOG.md)

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
