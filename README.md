# Rdp Sensei (Cross-Platform Web & Desktop RDP Platform)

Go dili ile geliştirilmiş, **hiçbir işletim sisteminde harici hiçbir programa (Mac'te Windows App / Microsoft Remote Desktop, Linux'ta Remmina / FreeRDP, Windows'ta mstsc) ihtiyaç duymayan**, tamamen kendi içinde çalışan modern RDP yönetim ve bağlantı platformu.

Tasarımı macOS **Microsoft Remote Desktop / Windows App** arayüzü ile birebir aynıdır.

---

## 🥋 Öne Çıkan Özellikler

1. **Sıfır Bağımlılık (Zero External Dependencies):**
   - RDP protokol motoru **Pure Go WebAssembly (WASM)** olarak doğrudan uygulama içerisindeki HTML5 Canvas üzerinde çalışır.
   - Go backend'i WebSocket ile raw TCP RDP (port 3389) arasında köprü kurar.
   - İşletim sistemine ekstra hiçbir RDP paketi, kütüphane veya istemci kurulması gerekmez.

2. **İnteraktif Kimlik Doğrulama & Windows NLA Desteği:**
   - Şifresi kayıtlı olmayan bir bilgisayara tıklandığında anında şifre sorma modalı açılır.
   - Şifreyi kaydetme (`Remember this password`) seçeneği sunulur.
   - NLA hatalarında net ve açıklayıcı hata mesajı gösterilir ("Kullanıcı adı/şifre hatalı, tekrar deneyin").

3. **macOS `.app` Paketi:**
   - Çift tıklayarak doğrudan yerel uygulama gibi çalıştırabileceğiniz **`Rdp Sensei.app`** paketi hazırdır. İsterseniz `/Applications` (Uygulamalar) klasörüne taşıyabilirsiniz.

4. **Çift Mod (Desktop & Web):**
   - **Masaüstü Modu:** macOS, Windows ve Linux üzerinde bağımsız, adres çubuğu olmayan yerel masaüstü penceresi olarak açılır.
   - **Web Sunucu Modu (`-web`):** Ağ üzerindeki herhangi bir bilgisayar, tablet veya telefondan `http://<ip>:8080` ile erişilebilir.

---

## 🛠️ Hızlı Başlatma

### 1. macOS Uygulaması Olarak Açma
Doğrudan klasördeki **`Rdp Sensei.app`** dosyasına çift tıklayarak açabilirsiniz veya terminalden:
```bash
open "Rdp Sensei.app"
```

### 2. Terminalden Başlatma
```bash
./rdp-sensei
```

### 3. Web Sunucusu Modunda Başlatma
```bash
./rdp-sensei -web -port 8080
```

---

## 📦 Çapraz Derleme ve Paketleme

```bash
make app         # macOS için Rdp Sensei.app üretir
make build-all   # Tüm platformlar için derler (build/ dizinine)
```

Çıktılar `build/` dizinindedir:
- `build/Rdp Sensei.app` (macOS Çift tıklanabilir uygulama paketi)
- `build/rdp-sensei-darwin-arm64` (macOS Apple Silicon)
- `build/rdp-sensei-darwin-amd64` (macOS Intel)
- `build/rdp-sensei-windows-amd64.exe` (Windows 10 / 11 / Server)
- `build/rdp-sensei-linux-amd64` (Linux)
