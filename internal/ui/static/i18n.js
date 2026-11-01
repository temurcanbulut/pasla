"use strict";

/* Translations for the desktop and phone pages.
   English is the source language and the default: the English text is the
   key, so a missing translation falls back to English. To add a language,
   add an entry to LANGS and a dictionary like TR below. */

(function () {
  const LANGS = [
    { code: "en", name: "English", short: "EN", locale: "en-GB" },
    { code: "tr", name: "Türkçe", short: "TR", locale: "tr-TR" },
  ];

  const TR = {
    "Paired device renamed: {old} → {name}": "Eşleşmiş cihazın adı değişti: {old} → {name}",
    // units and time
    "{n} s": "{n} sn",
    "{n} min": "{n} dk",
    "{h} h {m} min": "{h} sa {m} dk",
    "{m} min {s} s": "{m} dk {s} sn",
    "{time} left": "{time} kaldı",

    // shell
    "This device": "Bu cihaz",
    "Settings for this device": "Bu cihazın ayarları",
    "Received": "Alınanlar",
    "Settings": "Ayarlar",
    "Devices": "Cihazlar",
    "Devices on your network": "Ağdaki cihazlar",
    "Connect a phone": "Telefonu bağla",
    "Not in the list? Add by IP": "Listede yok mu? IP ile ekle",
    "Add": "Ekle",
    "Choose a device": "Bir cihaz seçin",
    "Computers on the same network with Pasla open appear in the list on the left. Pick one, pair once, then drag and drop your files.":
      "Aynı ağda Pasla açık olan bilgisayarlar soldaki listede belirir. Birini seçin, bir kez eşleştirin, sonra dosyaları sürükleyip bırakın.",
    "Unknown system": "Bilinmeyen sistem",
    "Online": "Çevrimiçi",
    "No session": "Oturum yok",
    "This window is not signed in": "Bu pencere oturum açmamış",
    "For your security the Pasla interface only works in the window the program opens itself. Start Pasla again, or click its shortcut again, and the interface opens on its own.":
      "Güvenliğiniz için Pasla arayüzü yalnızca programın kendisinin açtığı pencerede çalışır. Pasla'yı yeniden başlatın ya da kısayoluna yeniden tıklayın; arayüz kendiliğinden açılır.",
    "Cannot reach Pasla": "Pasla'ya bağlanılamıyor",
    "Listening on the network": "Ağ dinleniyor",
    "Automatic discovery is off": "Otomatik keşif kapalı",

    // notifications
    "Paired with {name}. You can send files now.": "{name} ile eşleşildi. Artık dosya gönderebilirsiniz.",
    "{name} wants to pair": "{name} eşleşmek istiyor",
    "{name} wants to connect": "{name} bağlanmak istiyor",
    "{name} sent a text": "{name} bir metin gönderdi",
    "{name} wants to send {n} file": "{name} {n} dosya göndermek istiyor",
    "{name} wants to send {n} files": "{name} {n} dosya göndermek istiyor",
    "Received {n} file from {name}": "{name} cihazından {n} dosya alındı",
    "Received {n} files from {name}": "{name} cihazından {n} dosya alındı",
    "Sent {n} file to {name}": "{name} cihazına {n} dosya gönderildi",
    "Sent {n} files to {name}": "{name} cihazına {n} dosya gönderildi",

    // device list
    "Offline": "Çevrimdışı",
    "Paired": "Eşleşmiş",
    "Not paired": "Eşleşmemiş",
    "Nobody here yet": "Henüz kimse görünmüyor",
    "Open Pasla on your other computers. They show up here within a few seconds.":
      "Diğer bilgisayarlarda Pasla'yı açın. Birkaç saniye içinde burada belirirler.",
    "Unnamed device": "Adsız cihaz",
    "Same name as another device!": "Aynı adla başka cihaz!",
    "Phones": "Telefonlar",
    "Connected from the browser": "Tarayıcıdan bağlı",
    "{n} unread": "{n} okunmamış",

    // device page
    "Possible impersonation": "Ad taklidi olabilir",
    "Back to the device list": "Cihaz listesine dön",
    "You already have a paired device called “{name}”": "Zaten “{name}” adlı eşleşmiş bir cihazınız var",
    "This device uses the same name but has a different identity. That is normal if the computer was reinstalled or Pasla's settings folder was deleted. Otherwise someone may be trying to pose as that device: ask the person at that computer before pairing.":
      "Bu cihaz aynı adı taşıyor ama kimliği farklı. Bilgisayar yeniden kurulduysa ya da Pasla'nın ayar klasörü silindiyse bu normaldir. Değilse biri o cihazın kılığına girmeye çalışıyor olabilir: eşleştirmeden önce o bilgisayarın başındaki kişiye sorun.",
    "Pair once to get started": "Önce bir kez eşleştirin",
    "Pairing makes sure your files only ever go to this exact device. The connection is always encrypted.":
      "Eşleştirme, dosyaların yalnızca gerçekten bu cihaza gitmesini sağlar. Bağlantı her zaman şifrelidir.",
    "Press Pair.": "Eşleştir'e basın.",
    "The same 6-digit code appears on both screens.": "İki ekranda da aynı 6 haneli kod görünür.",
    "If the codes match, confirm on both devices.": "Kodlar aynıysa iki cihazda da onaylayın.",
    "Pair": "Eşleştir",
    "Text and links": "Metin ve bağlantılar",
    "Transfers": "Aktarımlar",
    "Clear finished": "Bitenleri temizle",
    "About this device": "Bu cihazla ilgili",
    "Accept files from this device without asking": "Bu cihazdan gelenleri sormadan kabul et",
    "When on, what this device sends goes straight into your received folder.":
      "Açıkken bu cihazın gönderdikleri doğrudan alınanlar klasörüne kaydedilir.",
    "Safety number": "Güvenlik numarası",
    "The other device shows the same number. Whenever you want peace of mind, compare the two screens; if they differ, remove the pairing.":
      "Karşı cihazda da aynı numara görünür. İçiniz rahat etmek istediğinizde iki ekrandaki numarayı karşılaştırın; farklıysa eşleşmeyi kaldırın.",
    "Identity fingerprint": "Kimlik parmak izi",
    "Remove pairing": "Eşleşmeyi kaldır",
    "{addr} removed from manually added addresses": "{addr} elle eklenenlerden çıkarıldı",
    "Remove from manual addresses": "Elle eklenenlerden çıkar",
    "Send to {name}": "{name} cihazına gönder",
    "{name} is offline right now": "{name} şu anda çevrimdışı",
    "Drop files or folders here, or paste them (Ctrl+V).": "Dosya ya da klasörleri buraya bırakın, veya yapıştırın (Ctrl+V).",
    "You can send again once Pasla is open on that device.": "Cihazda Pasla açıldığında yeniden gönderebilirsiniz.",
    "Choose files": "Dosya seç",
    "Choose a folder": "Klasör seç",
    "Keep this tab open while sending.": "Gönderim sürerken bu sekmeyi kapatmayın.",
    "Type a text, note or link…": "Metin, not ya da bağlantı yazın…",
    "Text to send": "Gönderilecek metin",
    "Send text": "Metni gönder",
    "Copy": "Kopyala",
    "Copied": "Kopyalandı",
    "Type a password, link or note; it appears on the other screen right away.":
      "Bir şifre, bağlantı ya da not yazın; karşı ekranda anında belirir.",

    // transfers
    "Waiting for the other side to accept": "Karşı tarafın onayı bekleniyor",
    "Waiting for your answer": "Onayınızı bekliyor",
    "Sent {n} file": "Gönderildi: {n} dosya",
    "Sent {n} files": "Gönderildi: {n} dosya",
    "Received {n} file": "Alındı: {n} dosya",
    "Received {n} files": "Alındı: {n} dosya",
    "Declined": "Reddedildi",
    "Cancelled": "İptal edildi",
    "Could not finish: {why}": "Tamamlanamadı: {why}",
    "unknown error": "bilinmeyen hata",
    "Outgoing": "Giden",
    "Incoming": "Gelen",
    "Cancel": "İptal",
    "Show": "Göster",
    "{n} file": "{n} dosya",
    "{n} files": "{n} dosya",
    "Nothing has come from this phone yet.": "Bu telefondan henüz bir şey gelmedi.",
    "Nothing shared with this device yet.": "Bu cihazla henüz bir şey paylaşılmadı.",
    "{name} did not accept the files": "{name} gönderimi kabul etmedi",
    "Could not start sending": "Gönderim başlatılamadı",

    // incoming offer
    "{name} wants to send files": "{name} dosya göndermek istiyor",
    "{name} is sending files": "{name} dosya gönderiyor",
    "{n} file, {size} in total": "{n} dosya, toplam {size}",
    "{n} files, {size} in total": "{n} dosya, toplam {size}",
    "These files can run code (programs, scripts or installers). Decline if you were not expecting them.":
      "Bu dosyalar kod çalıştırabilir (program, betik ya da kurulum dosyası). Beklemiyorsanız reddedin.",
    "{n} file can run code (a program, script or installer). Decline if you were not expecting it.":
      "{n} dosya kod çalıştırabilir (program, betik ya da kurulum dosyası). Beklemiyorsanız reddedin.",
    "{n} files can run code (programs, scripts or installers). Decline if you were not expecting them.":
      "{n} dosya kod çalıştırabilir (program, betik ya da kurulum dosyası). Beklemiyorsanız reddedin.",
    "{n} file including folder contents": "klasör içerikleriyle birlikte {n} dosya",
    "{n} files including folder contents": "klasör içerikleriyle birlikte {n} dosya",
    "Decline": "Reddet",
    "Accept": "Kabul et",

    // pairing
    "Code {digits}": "Kod {digits}",
    "Compare the codes": "Kodları karşılaştırın",
    "{name} should be showing this same code. If it does, confirm.": "{name} ekranında da bu kod görünüyor olmalı. Aynıysa onaylayın.",
    "The same code just appeared on {name}. If they match, confirm on both devices.":
      "{name} ekranında da aynı kod belirdi. Aynıysa iki cihazda da onaylayın.",
    "with": "ile",
    "Careful: you are already paired with a device called “{name}”; this request comes from a different identity.":
      "Dikkat: “{name}” adlı cihazla zaten eşleşmişsiniz; bu istek farklı bir kimlikten geliyor.",
    "If the codes differ, decline: another device may be trying to get in between.":
      "Kodlar farklıysa reddedin: başka bir cihaz araya girmeye çalışıyor olabilir.",
    "Codes match, pair": "Kodlar aynı, eşleştir",
    "Almost done": "Neredeyse tamam",
    "Now confirm “Codes match” on {name} too.": "Şimdi {name} cihazında da “Kodlar aynı” deyin.",
    "Waiting for {name}…": "{name} onayı bekleniyor…",
    "Wait in the background": "Arka planda bekle",
    "Files from {name} will now be accepted without asking": "{name} cihazından gelenler artık sorulmadan alınacak",
    "You will be asked before accepting files from {name}": "{name} cihazından gelenler için onay istenecek",
    "Remove the pairing with {name}? To send files again you will need to pair again.":
      "{name} ile eşleşme kaldırılsın mı? Tekrar dosya göndermek için yeniden eşleştirmeniz gerekir.",
    "Pairing with {name} removed": "{name} ile eşleşme kaldırıldı",

    // settings
    "Remove": "Çıkar",
    "None. You can add devices that automatic discovery cannot reach by IP, in the box on the left.":
      "Yok. Otomatik keşfin ulaşamadığı cihazları soldaki kutudan IP ile ekleyebilirsiniz.",
    "Allow desktop notifications": "Masaüstü bildirimlerine izin ver",
    "Settings saved": "Ayarlar kaydedildi",
    "Other computers see this device under this name.": "Diğer bilgisayarlar bu cihazı bu adla görür.",
    "Device name": "Cihaz adı",
    "Language": "Dil",
    "Received files folder": "Alınanlar klasörü",
    "Incoming files are saved here. Nothing is overwritten: if a name is taken, “(2)” is added.":
      "Gelen dosyalar buraya kaydedilir. Hiçbir şeyin üzerine yazılmaz: aynı adlı bir şey varsa “(2)” eklenir.",
    "Accept files from all paired devices without asking": "Eşleşmiş tüm cihazlardan gelenleri sormadan kabul et",
    "When off, you are asked before every transfer.": "Kapalıyken her gönderim için onay istenir.",
    "Manually added addresses": "Elle eklenen adresler",
    "This device's identity. Pairing trusts this identity, so it keeps working when the IP address changes.":
      "Bu cihazın kimliği. Eşleştirme bu kimliğe güvenir; IP adresi değişse de bozulmaz.",
    "IP addresses": "IP adresleri",
    "None found": "Bulunamadı",
    "Transfer port": "Aktarım portu",
    "Automatic discovery": "Otomatik keşif",
    "On": "Açık",
    "Off": "Kapalı",
    "Version": "Sürüm",
    "Close": "Kapat",
    "Save": "Kaydet",
    "{name} added": "{name} eklendi",
    "Quit Pasla": "Pasla'yı kapat",
    "Keep running after I close the window": "Pencereyi kapatınca da çalışmaya devam etsin",
    "So other devices can send you files at any time. Pasla stays next to the clock; right-click its icon to quit. When off, closing the window quits Pasla.":
      "Böylece başka cihazlar size her zaman dosya gönderebilir. Pasla saatin yanında kalır; kapatmak için simgesine sağ tıklayın. Kapalıyken pencereyi kapatmak Pasla'yı da kapatır.",
    "So other devices can send you files at any time. When off, closing the window quits Pasla.":
      "Böylece başka cihazlar size her zaman dosya gönderebilir. Kapalıyken pencereyi kapatmak Pasla'yı da kapatır.",
    "Transfers are still running. Quit Pasla anyway?": "Aktarımlar hâlâ sürüyor. Yine de Pasla kapatılsın mı?",
    "Pasla is closed": "Pasla kapatıldı",
    "You can close this tab. To use Pasla again, start it the same way you did before.":
      "Bu sekmeyi kapatabilirsiniz. Pasla'yı yeniden kullanmak için önceki gibi tekrar başlatın.",

    // phones (desktop side)
    "Send to the phone": "Telefona gönder",
    "Drop files here. They appear in the phone's “From the computer” list, ready to download.":
      "Dosyaları buraya bırakın. Telefonda “Bilgisayardan gelenler” listesinde belirir, oradan indirilir.",
    "To the phone": "Telefona",
    "downloaded on the phone": "telefona indirildi",
    "waiting to be downloaded on the phone": "telefonda indirilmeyi bekliyor",
    "Remove from list": "Listeden kaldır",
    "Nothing sent to the phone yet.": "Telefona henüz bir şey gönderilmedi.",
    "Approved": "Onaylı",
    "Phone connected from the browser": "Tarayıcıdan bağlı telefon",
    "Sent to the phone": "Telefona gönderilenler",
    "From the phone": "Telefondan gelenler",
    "{name} disconnected": "{name} bağlantısı kesildi",
    "Disconnect": "Bağlantıyı kes",
    "{file} is ready on the phone": "{file} telefona hazır",
    "{n} file is ready on the phone": "{n} dosya telefona hazır",
    "{n} files are ready on the phone": "{n} dosya telefona hazır",
    "{name} connected": "{name} bağlandı",
    "Does the phone show the same number?": "Telefon ekranındaki numara bununla aynı mı?",
    "Same, allow": "Aynı, izin ver",
    "The phone must be on the same Wi-Fi as this computer. No app needed.":
      "Telefon bu bilgisayarla aynı Wi-Fi ağında olmalı. Uygulama kurmaya gerek yok.",
    "Network address": "Ağ adresi",
    "Not scanning? Try another address:": "Okunmuyorsa başka bir adres deneyin:",
    "Scan the QR code with the phone's camera.": "Telefonun kamerasıyla QR kodu okutun.",
    "If the browser says the connection is not private, tap Advanced, then Proceed. The connection is still encrypted; the warning appears because this computer made its own certificate instead of buying one from a company.":
      "Tarayıcı “bağlantınız gizli değil” derse Gelişmiş'e, sonra Devam et'e dokunun. Bağlantı yine şifrelidir; uyarı, sertifikayı bir şirketten almak yerine bu bilgisayarın kendisinin üretmesinden kaynaklanır.",
    "The same 4-digit number appears on the phone and here; if it matches, allow it.":
      "Telefonda ve burada aynı 4 haneli numara çıkar; aynıysa izin verin.",
    "The code expires in {time}.": "Kod {time} sonra geçersiz olur.",
    "The code has expired. Create a new one.": "Kodun süresi doldu. Yeni bir kod oluşturun.",
    "Turn off phone connections": "Telefon bağlantılarını kapat",
    "New code": "Yeni kod",
    "Done": "Tamam",

    // phone page
    "Android phone": "Android telefon",
    "Phone": "Telefon",
    "Connect": "Bağlan",
    "Connect to the computer": "Bilgisayara bağlan",
    "This phone talks straight to the computer on the same network. Nothing goes to the internet.":
      "Bu telefon, aynı ağdaki bilgisayarla doğrudan konuşur. Hiçbir şey internete gitmez.",
    "Name shown on the computer": "Bilgisayarda görünecek ad",
    "Not connected": "Bağlantı yok",
    "On the computer, open Pasla, press “Connect a phone” and scan the QR code again.":
      "Bilgisayarda Pasla'yı açın, “Telefonu bağla”ya basın ve çıkan QR kodu yeniden okutun.",
    "Confirm on the computer": "Bilgisayarda onaylayın",
    "A prompt just opened on {computer}. If the number there matches this one, press “Same, allow”.":
      "{computer} ekranında bir onay kutusu açıldı. Oradaki numara bununla aynıysa “Aynı, izin ver”e basın.",
    "Check number {digits}": "Kontrol numarası {digits}",
    "Waiting for approval": "Onay bekleniyor",
    "Send to {computer}": "{computer} bilgisayarına gönder",
    "Photos, videos or any file. They are saved in the computer's received files folder.":
      "Fotoğraf, video ya da herhangi bir dosya. Bilgisayardaki Alınanlar klasörüne kaydedilir.",
    "Sending…": "Gönderiliyor…",
    "Choose files and send": "Dosya seç ve gönder",
    "From the computer": "Bilgisayardan gelenler",
    "downloaded": "indirildi",
    "Download": "İndir",
    "Files the computer sends to this phone appear here.": "Bilgisayardan bu telefona gönderilenler burada belirir.",
    "Type a text, link or note for the computer": "Bilgisayara metin, bağlantı ya da not yazın",
    "Text": "Metin",
    "From the computer, {time}": "Bilgisayardan, {time}",
    "Sent, {time}": "Gönderildi, {time}",
    "You sent": "Gönderdiklerin",
    "Not accepted": "Kabul edilmedi",
    "{n} file, {size}, saved": "{n} dosya, {size}, kaydedildi",
    "{n} files, {size}, saved": "{n} dosya, {size}, kaydedildi",
    "Connected to {computer}": "{computer} ile bağlı",
    "The connection is encrypted. It closes by itself when you close this page or after 30 minutes without use.":
      "Bağlantı şifreli. Bu sayfayı kapattığınızda ya da 30 dakika kullanmadığınızda kendiliğinden kapanır.",
    "The connection dropped": "Bağlantı koptu",
    "Sent": "Gönderildi",
    "{n} file sent": "{n} dosya gönderildi",
    "{n} files sent": "{n} dosya gönderildi",
    "Disconnected": "Bağlantı kesildi",
    "To connect again, scan the QR code on the computer.": "Yeniden bağlanmak için bilgisayardaki QR kodu okutun.",
  };

  const DICTS = { tr: TR };
  const KEY = "pasla.lang";

  let current = "en";
  try { current = localStorage.getItem(KEY) || "en"; } catch { /* storage unavailable */ }
  if (!LANGS.some((l) => l.code === current)) current = "en";

  function fill(s, vars) {
    if (!vars) return s;
    return s.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m));
  }

  function t(s, vars) {
    const d = DICTS[current];
    return fill((d && d[s]) || s, vars);
  }

  // tn picks the English singular or plural form; the chosen form is then
  // translated like any other text (Turkish uses one form for both).
  function tn(n, one, other, vars) {
    return t(n === 1 ? one : other, Object.assign({ n }, vars));
  }

  function setLang(code) {
    if (!LANGS.some((l) => l.code === code)) code = "en";
    current = code;
    try { localStorage.setItem(KEY, code); } catch { /* storage unavailable */ }
    document.documentElement.lang = code;
  }

  // Elements in the static HTML carry their English text in data-i18n
  // attributes, so the page can be translated without rebuilding it.
  function applyDOM(root) {
    for (const el of root.querySelectorAll("[data-i18n]")) el.textContent = t(el.dataset.i18n);
    for (const el of root.querySelectorAll("[data-i18n-title]")) el.title = t(el.dataset.i18nTitle);
    for (const el of root.querySelectorAll("[data-i18n-aria-label]")) el.setAttribute("aria-label", t(el.dataset.i18nAriaLabel));
    for (const el of root.querySelectorAll("[data-i18n-placeholder]")) el.placeholder = t(el.dataset.i18nPlaceholder);
  }

  function locale() { return LANGS.find((l) => l.code === current).locale; }

  function bytes(n) {
    if (!n) return "0 B";
    const u = ["B", "KB", "MB", "GB", "TB"];
    let i = 0;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    const num = i === 0 ? String(n) : new Intl.NumberFormat(locale(), { maximumFractionDigits: 1 }).format(n);
    return num + " " + u[i];
  }

  function time(d) {
    return new Intl.DateTimeFormat(locale(), { hour: "2-digit", minute: "2-digit" }).format(d);
  }

  document.documentElement.lang = current;
  window.I18N = { t, tn, setLang, lang: () => current, applyDOM, bytes, time, languages: LANGS, dictionaries: DICTS };
})();
