# Deployment

UI memakai `network_mode: host` pada Linux: alamat loopback UI adalah loopback host. Tidak perlu pemetaan `ports`. Build juga memakai jaringan host. Container berjalan non-root, filesystem read-only, tanpa capability dan tanpa mount socket Docker.

## Pengamanan server VPN/cloud

Installer Docker dapat membuat bridge dan aturan firewall. Pada VM cloud, link virtual juga dapat memicu cloud-init hotplug dan menjalankan ulang DHCP. Jangan langsung menjalankan installer di server VPN aktif tanpa pengamanan.

1. Simpan baseline default route, policy rules, resolver, dan jalur SSH. Siapkan akses konsol serta rollback independen.
2. Pada image yang memiliki cloud-init hotplug, kecualikan `docker0`, `veth*`, dan `br-*` dari hook hotplug sebelum Docker dimulai. Ikuti [panduan backend](https://github.com/ica4me/govpn-wg/tree/main/docs). Jangan menghapus aturan cloud-init untuk NIC fisik.
3. Pada host khusus UI/VPN tanpa kebutuhan bridge Docker lain, buat konfigurasi berikut **sebelum Docker di-install atau dimulai** di `/etc/docker/daemon.json`:

```json
{
  "bridge": "none",
  "iptables": false,
  "ip6tables": false,
  "ip-forward": false,
  "ip-masq": false
}
```

Jika file sudah ada, gabungkan setting; jangan menimpa konfigurasi container lain. Setting ini daemon-wide dan dapat memengaruhi container bridge lain. Jangan restart daemon yang menampung workload lain tanpa menilai dampaknya.

4. Setelah pemasangan dan start UI, bandingkan default route, policy rules dan DNS dengan baseline serta pastikan SSH dan VPN tetap terhubung. UI tidak memerlukan akses administrasi jaringan host.

## Backend pada server terpisah

- `GOVPN_API_URL` selalu diakses oleh proses UI, bukan oleh browser pengguna.
- Gunakan IP privat management atau endpoint HTTPS yang sertifikatnya valid. Jangan memakai alamat tunnel yang dibatasi firewall VPN untuk administrasi hub.
- Backend default hanya listen loopback. Ubah `GOVPN_LISTEN` di `/etc/govpn/wg.env` menjadi IP privat dan port API yang sesuai, misalnya `10.0.0.10:8080`.
- Batasi firewall/security group ke IP server UI. Jangan mengekspos API HTTP ke internet. Perubahan listen memerlukan restart backend; jadwalkan dengan pengaman dan cek konektivitas.
- Periksa dari host UI: `curl -fsS http://10.0.0.10:8080/api/v1/health`.

## HTTPS

Gunakan reverse proxy TLS di depan `127.0.0.1:8090`, teruskan Host asli, dan set `GOVPN_COOKIE_SECURE: 'true'`. Jangan mengaktifkan secure cookie jika browser masih menggunakan HTTP. Untuk API HTTPS, sertakan CA certificate bundle tepercaya di image runtime; image scratch bawaan belum membawa bundle CA. Jangan mematikan verifikasi TLS.

## Opsional: Nginx Proxy Manager (NPM) dan pembatasan login

Ini konfigurasi opsional untuk proxy **yang sudah tersedia**, bukan instruksi memasang NPM atau mengganti jaringan Docker. Compose bawaan tetap HTTP loopback dengan secure cookie `false`; instalasi baru tetap dapat login langsung melalui HTTP untuk pengujian.

1. Pada Proxy Host NPM, pilih domain UI, forward scheme `http`, host/IP UI yang memang dapat dijangkau NPM, port `8090`. Pertahankan Host asli agar pemeriksaan Origin/CSRF bekerja. Jika NPM di container bridge, `127.0.0.1` adalah container NPM sendiri, bukan host UI. Tentukan jalur yang aman sesuai instalasi Anda; jangan mengubah routing/jaringan host hanya demi contoh ini.
2. Aktifkan sertifikat TLS dan Force SSL di NPM. Setelah browser benar-benar memakai HTTPS, set `GOVPN_COOKIE_SECURE=true` pada environment UI dan recreate UI. Jangan aktifkan pada akses browser HTTP: cookie tidak terkirim dan login akan gagal. HTTPS di browser cukup meskipun hop privat NPM→UI HTTP.
3. Default UI mengabaikan `X-Forwarded-For`, `X-Real-IP`, dan `Forwarded`. Untuk membedakan pengguna di belakang NPM, set `GOVPN_TRUSTED_PROXY_CIDRS` ke **IP peer NPM yang benar-benar terlihat oleh UI**, misalnya `10.0.0.2/32` atau `2001:db8::2/128`; daftar dipisahkan koma. Jangan salin IP contoh tanpa verifikasi. Jangan percayai subnet semua pengguna, semua container, atau internet; catch-all `/0` ditolak dan CIDR invalid menghentikan startup. Kosong berarti tidak ada proxy tepercaya.
4. NPM harus menimpa XFF dengan IP client (`proxy_set_header X-Forwarded-For $remote_addr;`) atau menambahkan peer asli (`$proxy_add_x_forwarded_for`). UI memeriksa peer socket terlebih dahulu lalu membaca XFF dari kanan, berhenti pada hop pertama yang tidak dipercaya; header client palsu di kiri tidak dapat memilih bucket limiter. Header malformed kembali ke peer socket. Jika ada proxy/CDN tambahan, percayai hanya hop yang dikendalikan dan pastikan setiap hop menambahkan/menimpa IP secara benar. Jangan memakai konfigurasi real-IP Nginx yang mempercayai internet.
5. Atur sendiri IP allowlist, NPM Access List, dan firewall sesuai kebutuhan. Jangan membuka API backend untuk publik. Ini tidak memasang allowlist atau mengubah aturan jaringan.

UI membatasi sebelum meneruskan `/auth/login` ke backend: **5 percobaan per IP per jendela 1 menit**, maksimal **4 login backend aktif** per proses, dan **10.000 bucket IP**. Percobaan sukses juga dihitung; bucket penuh menolak client baru sampai kedaluwarsa (tidak menggusur bucket aktif). Respons `429` menampilkan pesan retry dan `Retry-After` dalam detik. CSRF tetap diperiksa sebelum admission. Limit/sesi di RAM hilang saat restart; beberapa replika perlu limit terpusat di proxy. UI tidak melindungi akses langsung API backend, jadi jangan melewati UI/proxy untuk endpoint login publik.

Sebagai lapisan tambahan, rate limit Nginx memakai `$binary_remote_addr` (bukan XFF mentah). Pada konfigurasi **http-context** NPM yang didukung instalasi Anda (misalnya `/data/nginx/custom/http_top.conf`), definisikan:

```nginx
limit_req_zone $binary_remote_addr zone=govpn_login:10m rate=5r/m;
```

Lalu pada location `/login` yang melayani UI (Custom Location NPM atau konfigurasi location yang dikelola sendiri), tambahkan:

```nginx
limit_req zone=govpn_login burst=5 nodelay;
limit_req_status 429;
```

Pertahankan konfigurasi proxy pass, Host, dan XFF pada location tersebut. `limit_req_zone` tidak boleh diletakkan di field Advanced server/location. Contoh ini menghitung GET dan POST `/login`; UI membatasi POST autentikasi. Nginx tidak otomatis memberi `Retry-After`; untuk halaman 429 proxy, gunakan error handler lokal dengan header `Retry-After: 60` sesuai konfigurasi Nginx Anda. Jalankan `nginx -t` di lingkungan NPM sebelum reload, cek HTTPS/login/CSRF serta 429, dan rollback hanya konfigurasi proxy jika validasi gagal. Jangan mengganti konfigurasi NPM yang dihasilkan secara buta.

Pada backend baru, respons login `requires_password_change:true` membatasi sesi UI hanya ke `/account/password` dan logout; dashboard, download dan operasi lain dialihkan tanpa request backend. Ganti password bootstrap, lalu login ulang dengan password baru. Backend lama yang tidak mengirim field tersebut tetap kompatibel (default false). Backend juga harus menegakkan kebijakan ini untuk akses API langsung.

## Operasional

```bash
sudo docker compose logs --tail=100 ui
sudo docker compose ps
sudo docker compose restart ui
sudo docker compose down
```

Restart/recreate UI menghapus sesi RAM: pengguna perlu login ulang, tetapi state VPN backend tidak terhapus. Jangan mengubah default gateway atau DNS host untuk menjalankan UI. UI tidak memerlukan database sendiri.
