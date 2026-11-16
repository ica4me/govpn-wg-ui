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

## Operasional

```bash
sudo docker compose logs --tail=100 ui
sudo docker compose ps
sudo docker compose restart ui
sudo docker compose down
```

Restart/recreate UI menghapus sesi RAM: pengguna perlu login ulang, tetapi state VPN backend tidak terhapus. Jangan mengubah default gateway atau DNS host untuk menjalankan UI. UI tidak memerlukan database sendiri.
