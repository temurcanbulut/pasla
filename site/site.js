// Language switch (English in the HTML, Turkish below) and the download
// button that matches the visitor's system. No cookies, no trackers: the
// language choice is kept in localStorage only.
(() => {
  "use strict";

  const TR = {
    "title": "Pasla: yerel ağınız için çevrimdışı dosya paylaşımı",
    "description": "Yerel ağınızdaki bilgisayarlar ve telefonlar arasında dosya, klasör ve metin gönderin. İnternet, hesap ya da bulut gerekmez. Ücretsiz ve açık kaynak; Windows, Linux ve her telefon tarayıcısı için.",

    "nav.how": "Nasıl çalışır",
    "nav.phones": "Telefonlar",
    "nav.security": "Güvenlik",
    "nav.faq": "SSS",

    "hero.eyebrow": "Tek dosya. Senin ağın.",
    "hero.title": "Dosyayı yanındaki bilgisayara pasla.",
    "hero.lede": "Bilgisayarlarınız ve telefonlarınız arasında dosya, klasör ve metin gönderin. Doğrudan yerel ağınızdan, cihazdan cihaza şifreli.",
    "hero.dlWin": "Windows için indir",
    "hero.dlLinux": "Linux için indir",
    "hero.other": "Tüm indirmeler",
    "hero.fine": "Ücretsiz ve açık kaynak. Windows, Linux ve telefon tarayıcıları.",

    "why.1": "Hesap yok",
    "why.2": "Bulut yok",
    "why.3": "Şifreli aktarım",


    "preview.desktop": "Bilgisayar",
    "preview.phone": "Telefon",
    "preview.pairing": "Bağlantı",
    "preview.real": "Uygulamanın kendisi.",
    "preview.open": "Tam görüntüyü aç",
    "preview.title": "Pasla'nın içinden",
    "preview.close": "Kapat",

    "how.title": "Nasıl çalışır",
    "how.lede": "İlk seferde üç adım. Sonrasında sadece sürükleyip bırakın.",
    "how.1.t": "İki bilgisayarda da açın",
    "how.1.p": "Dosyayı indirip çift tıklayın. Arayüz tarayıcınızda açılır; aynı ağda Pasla çalışan bilgisayarlar birkaç saniye içinde listede belirir.",
    "how.2.t": "Bir kez eşleştirin",
    "how.2.p": "İki ekranda da aynı 6 haneli kod görünür. Aynıysa iki tarafta da onaylayın. Bundan sonra iki cihaz, IP adresleri değişse bile birbirini tanır.",
    "how.3.t": "Sürükleyip bırakın",
    "how.3.p": "Dosyaları ya da klasörleri cihazın üstüne bırakın. Karşı taraf kabul edince aktarım başlar. Klasör yapısı korunur, hiçbir şeyin üzerine yazılmaz.",

    "phones.title": "Telefonlar da, uygulamasız",
    "phones.p1": "Bilgisayarda <b>Telefonu bağla</b>'ya basıp QR kodu okutun. İki ekranda da aynı 4 haneli numara çıkar; aynıysa bilgisayarda izin verin.",
    "phones.p2": "Zaten kullandığınız tarayıcıdan iki yöne de fotoğraf, dosya ve metin gönderin. iPhone'da da Android'de de çalışır.",
    "phones.l1": "Telefona hiçbir şey kurulmaz",
    "phones.l2": "Telefon portu yalnızca gerektiğinde açılır",
    "phones.l3": "Kullanılmayan telefonun bağlantısı 30 dakika sonra kapanır",

    "feat.title": "Küçük program, gereken her şey",
    "feat.1.t": "Cihazları kendisi bulur",
    "feat.1.p": "Aynı ağdaki bilgisayarlar kendiliğinden görünür. Başka alt ağdaki cihazlar bir kez IP ile eklenir.",
    "feat.2.t": "Dosya, klasör, metin",
    "feat.2.p": "Koca klasör ağaçları, panodan yapıştırılan ekran görüntüleri ya da karşı ekranda kopyalanacak bir bağlantı veya not.",
    "feat.3.t": "Ağınız ne kadar hızlıysa",
    "feat.3.p": "Dosyalar doğrudan diske akar, varınca SHA-256 ile doğrulanır. Birden çok dosya aynı anda gider.",
    "feat.4.t": "Gerçekten çevrimdışı",
    "feat.4.p": "Arayüz, yazı tipleri ve simgeler programın içinde. Pasla ağınızın dışındaki hiçbir yere bağlanmaz.",
    "feat.5.t": "Ne geleceğine siz karar verin",
    "feat.5.p": "Güvendiğiniz bir cihaza izin vermediyseniz her aktarım önce size sorulur. Program dosyaları ayrıca işaretlenir.",
    "feat.6.t": "İsterseniz komut satırı",
    "feat.6.p": "<code>pasla send -to Ofis D:\\Fotoğraflar</code> çalışan Pasla üzerinden gönderir; betikler ve büyük işler için.",

    "sec.title": "Ağınızda gönül rahatlığıyla",
    "sec.lede": "Yerel ağ kendiliğinden güvenli değildir. Pasla, ağda başka birinin de olabileceğini varsayar.",
    "sec.1.t": "Şifreli",
    "sec.1.p": "Masaüstü eşleri karşılıklı TLS 1.3 ve parmak izi denetimi kullanır. Telefon TLS kullanır; tarayıcı başlangıcı aktif müdahaleye karşı güvenilir LAN gerektirir.",
    "sec.2.t": "Doğrulanmış eşleştirme",
    "sec.2.p": "Masaüstü eşleşmesinde iki ekrandaki 6 haneli kod karşılaştırılır. Sonraki masaüstü bağlantıları cihaz adı veya IP yerine sertifika parmak izini denetler.",
    "sec.3.t": "Klasörün dışına çıkmaz",
    "sec.3.p": "Gelen dosya adları temizlenir, bu kod fuzz testinden geçer. Windows'ta gelen dosyalara tarayıcı indirmeleri gibi Web işareti eklenir.",
    "sec.4.t": "Yalnızca siz kullanırsınız",
    "sec.4.p": "Arayüze yalnızca kendi bilgisayarınızdan ulaşılır, oturum anahtarı ister ve başka web sitelerinden gelen istekleri reddeder.",
    "sec.5.t": "Tahmini zor, ömrü kısa",
    "sec.5.p": "Telefon QR anahtarı 10 dakika geçerlidir ve bir kez kullanılır. Tahmin sınırı kaynak IP başınadır; her telefon ayrıca bilgisayarda onaylanır. Aktif araya girmeye karşı telefon kanalı güvenilir LAN gerektirir.",
    "sec.6.t": "Hesap yok, takip yok",
    "sec.6.p": "Sunucu da telemetri de yok. Pasla'nın bildiği her şey bilgisayarınızdaki bir klasörde kalır.",
    "sec.proto": "Protokol ve tehdit modeli",
    "sec.report": "Güvenlik açığı bildirin",

    "dl.title": "İndir",
    "dl.lede": "Sisteminize uygun dosyayı seçin. Ağınızda internet yoksa bir kez USB bellekle taşıyın.",
    "dl.win": "Windows 10 ve 11",
    "dl.linux": "Her dağıtım, bağımlılık yok",
    "dl.x64": "64 bit (x64)",
    "dl.arm": "ARM64 (Raspberry Pi 4/5)",
    "dl.phone.t": "Telefonlar",
    "dl.phone.p": "İndirilecek bir şey yok. Pasla çalışan bir bilgisayarın gösterdiği QR kodu okutun.",
    "dl.n1": "<b>Windows:</b> Dosyaya çift tıklayın. Arayüz tarayıcıda açılır, saatin yanında bir Pasla simgesi belirir. İlk açılışta güvenlik duvarı sorarsa <b>Özel ağlar</b> için izin verin.",
    "dl.n2": "<b>Linux:</b> <code>chmod +x pasla-linux-amd64 &amp;&amp; ./pasla-linux-amd64</code>",
    "dl.n3": "İndirdiğiniz dosyayı <a href=\"https://github.com/temurcanbulut/pasla/releases/tag/v0.4.3-beta.1\">sürüm sayfasındaki</a> <code>SHA256SUMS.txt</code> ile karşılaştırabilir ya da Go ile <a href=\"https://github.com/temurcanbulut/pasla/blob/main/README.tr.md\">kendiniz derleyebilirsiniz</a>.",
    "dl.setup": "Linux / Kali: hızlı başlangıç",
    "dl.setupIntro": "x86_64 (amd64) bir cihazda bu komutları terminale yapıştırın. Program yalnızca SHA256 doğrulaması başarılıysa başlar. Root ya da Go kurulumu gerekmez.",
    "dl.setupNetwork": "Terminali açık tutun. Aynı yerel ağdaki diğer bilgisayarda da Pasla'yı açın, iki ekrandaki eşleştirme kodunu karşılaştırın ve iki yöne de küçük bir dosya gönderin. Sanal makinede otomatik keşif için ağ bağdaştırıcısını köprü (bridged) modunda kullanın.",

    "faq.title": "Sorular",
    "faq.1.q": "Pasla internet bağlantısı istiyor mu?",
    "faq.1.a": "Hayır. Cihazlarınızın aynı ağda, yani aynı modem ya da switch üzerinden kabloyla veya Wi-Fi ile bağlı olması yeter. Pasla hiç internet olmayan ağlarda da çalışır.",
    "faq.2.q": "Gerçekten ücretsiz mi?",
    "faq.2.a": "Evet. Pasla MIT lisanslı açık kaynaktır. Hesap, reklam, ücretli sürüm ya da takip yoktur.",
    "faq.3.q": "Windows dosya için uyarı veriyor. Güvenli mi?",
    "faq.3.a": "Windows imzasız ikililer için uyarı gösterebilir. Çalıştırmadan önce açık kaynağı ve yayın kökenini inceleyin; <code>SHA256SUMS.txt</code> indirilen baytları doğrular.",
    "faq.4.q": "Telefonum neden bağlantının gizli olmadığını söylüyor?",
    "faq.4.a": "Bilgisayarınız sertifikasını kendisi üretti; telefonun tarayıcısı bunu herkese açık bir otoriteyle doğrulayamaz. Bağlantı yine de şifrelidir; doğru bilgisayara bağlandığınızı karşılaştırdığınız 4 haneli numara kanıtlar.",
    "faq.5.q": "Pencereyi kapatınca ne olur?",
    "faq.5.a": "Pasla birkaç saniye sonra kapanır; süren aktarımlar önce biter. Pencere kapalıyken de başka cihazlar size dosya gönderebilsin istiyorsanız Ayarlar'da <b>Pencereyi kapatınca da çalışmaya devam etsin</b>'i açın.",
    "faq.6.q": "macOS sürümü var mı?",
    "faq.6.a": "Henüz yok. O zamana kadar bir Mac de telefon gibi bağlanabilir: <b>Telefonu bağla</b>'nın gösterdiği QR kodu okutun ya da bağlantıyı herhangi bir tarayıcıda açın.",
    "faq.7.q": "Cihazlarım farklı alt ağlarda.",
    "faq.7.a": "Modemler, Pasla'nın cihaz bulmak için kullandığı yayınları alt ağlar arasında iletmez. Diğer cihazı bir kez IP adresiyle ekleyin; eşleştikten sonra Pasla onu bilinen son adresinde arar.",

    "foot.about": "Pasla, Türkçedeki “paslamak” fiilinden gelir. MIT lisanslı açık kaynak.",
    "foot.releases": "Sürümler",
    "foot.changes": "Değişiklikler",
    "foot.issues": "Sorun bildir",
    "foot.note": "Bu site çerez ya da takip aracı kullanmaz. Dil seçiminiz yalnızca tarayıcınızda saklanır.",
  };

  const EN = new Map();
  const meta = document.querySelector('meta[name="description"]');
  const isLinux = /Linux|X11/.test(navigator.userAgent) && !/Android/.test(navigator.userAgent);
  let lang = "en";
  let preview = "desktop";

  function stored() {
    try { return localStorage.getItem("pasla.site.lang"); } catch { return null; }
  }

  function text(key) {
    if (lang === "tr" && key in TR) return TR[key];
    return EN.get(key);
  }

  function apply(next) {
    lang = next === "tr" ? "tr" : "en";
    document.documentElement.lang = lang;
    document.querySelectorAll("[data-i]").forEach((el) => {
      const key = el.dataset.i;
      if (!EN.has(key)) EN.set(key, el.innerHTML);
      el.innerHTML = text(key);
    });
    if (!EN.has("title")) { EN.set("title", document.title); EN.set("description", meta.content); }
    document.title = text("title");
    meta.content = text("description");
    document.querySelectorAll("img[data-src-tr], img[data-alt-tr]").forEach((img) => {
      if (!img.dataset.srcEn) { img.dataset.srcEn = img.getAttribute("src"); img.dataset.altEn = img.alt; }
      img.src = lang === "tr" && img.dataset.srcTr ? img.dataset.srcTr : img.dataset.srcEn;
      img.alt = lang === "tr" && img.dataset.altTr ? img.dataset.altTr : img.dataset.altEn;
    });
    document.querySelectorAll("a[data-href-tr]").forEach((link) => {
      if (!link.dataset.hrefEn) link.dataset.hrefEn = link.getAttribute("href");
      link.href = lang === "tr" ? link.dataset.hrefTr : link.dataset.hrefEn;
    });
    document.querySelectorAll("[data-label-tr]").forEach((el) => {
      if (!el.dataset.labelEn) el.dataset.labelEn = el.getAttribute("aria-label");
      el.setAttribute("aria-label", lang === "tr" ? el.dataset.labelTr : el.dataset.labelEn);
    });
    document.querySelectorAll(".lang button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.lang === lang)));
    downloadButton();
    updatePreview();
  }

  const previewLink = document.getElementById("preview-open");
  const screenshotDialog = document.querySelector(".screenshot-dialog");
  const fullScreenshot = document.getElementById("screenshot-full");

  function updatePreview() {
    document.querySelectorAll(".preview-screen").forEach((screen) => { screen.hidden = screen.id !== "preview-" + preview; });
    document.querySelectorAll("[data-preview]").forEach((button) => {
      button.setAttribute("aria-pressed", String(button.dataset.preview === preview));
    });
    const image = document.querySelector("#preview-" + preview + " img");
    previewLink.href = image.src;
    if (screenshotDialog.open) {
      fullScreenshot.src = image.src;
      fullScreenshot.alt = image.alt;
    }
  }
  document.querySelectorAll("[data-preview]").forEach((button) => button.addEventListener("click", () => {
    preview = button.dataset.preview;
    updatePreview();
  }));
  document.querySelector(".preview-tabs").hidden = false;
  previewLink.addEventListener("click", (event) => {
    if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || typeof screenshotDialog.showModal !== "function") return;
    event.preventDefault();
    const image = document.querySelector("#preview-" + preview + " img");
    fullScreenshot.src = image.src;
    fullScreenshot.alt = image.alt;
    screenshotDialog.showModal();
    document.body.classList.add("previewing");
  });
  screenshotDialog.querySelector(".dialog-close").addEventListener("click", () => screenshotDialog.close());
  screenshotDialog.addEventListener("click", (event) => { if (event.target === screenshotDialog) screenshotDialog.close(); });
  screenshotDialog.addEventListener("close", () => document.body.classList.remove("previewing"));

  function downloadButton() {
    const label = document.getElementById("dl-main-label");
    if (!isLinux || !label) return;
    document.getElementById("dl-main").href = "https://github.com/temurcanbulut/pasla/releases/download/v0.4.3-beta.1/pasla-linux-amd64";
    if (!EN.has("hero.dlLinux")) EN.set("hero.dlLinux", "Download for Linux");
    label.dataset.i = "hero.dlLinux";
    label.innerHTML = text("hero.dlLinux");
  }

  document.querySelectorAll(".lang button").forEach((b) => b.addEventListener("click", () => {
    apply(b.dataset.lang);
    try { localStorage.setItem("pasla.site.lang", lang); } catch { /* private mode */ }
    const url = new URL(location.href);
    url.searchParams.delete("lang");
    history.replaceState(null, "", url);
  }));

  const asked = new URLSearchParams(location.search).get("lang");
  const prefersTr = (navigator.languages || [navigator.language || ""]).some((l) => /^tr\b/i.test(l));
  const first = asked || stored() || (prefersTr ? "tr" : "en");
  if (first === "tr") apply("tr");
  else { downloadButton(); updatePreview(); }
})();
