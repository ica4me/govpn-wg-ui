# Panduan Cepat: Backend + UI dalam 1 VM

Panduan ini untuk pemula. Ikuti dari atas ke bawah di **VM Linux baru**, misalnya Debian 12/13 atau Ubuntu 22.04/24.04. Hasil akhirnya:

- backend `govpn-wg` jalan sebagai service,
- UI bisa dibuka di `http://IP-VM:8090`,
- login pertama memakai **admin / admin**, lalu kamu langsung diminta membuat password baru.

> Mode ini untuk **uji coba**: UI dibuka langsung lewat HTTP di port 8090. Untuk dipakai sungguhan, taruh UI di belakang HTTPS (lihat [README](../README.md), opsi B).

Semua perintah dijalankan sebagai **root**. Kalau kamu login sebagai user biasa, jalankan `sudo -i` dulu.

---

## Langkah 1 — Update sistem

```bash
apt update && apt -y upgrade
apt -y install git curl ca-certificates
```

Kalau muncul file `/var/run/reboot-required`, reboot dulu (`reboot`), lalu login lagi.

## Langkah 2 — Pasang backend

```bash
cd /root
git clone https://github.com/ica4me/govpn-wg.git
cd govpn-wg
bash deploy/cloud-init-guard.sh      # aman untuk VM cloud; kalau tidak perlu, script langsung keluar
bash scripts/install.sh --yes
```

Proses ini kurang lebih **2 menit**. Go diunduh otomatis. Kalau berhasil, muncul tulisan `GoVPN is running.`

Cek:

```bash
curl -s http://127.0.0.1:8080/api/v1/health
# {"status": "ok", "version": "v1.5.0"}
```

## Langkah 3 — Pasang Docker dengan aman

Buat file ini **sebelum** memasang Docker. Isinya membuat Docker tidak mengubah jaringan dan firewall VM:

```bash
mkdir -p /etc/docker
cat > /etc/docker/daemon.json <<'EOF'
{
  "bridge": "none",
  "iptables": false,
  "ip6tables": false,
  "ip-forward": false,
  "ip-masq": false
}
EOF
```

Kalau `/etc/docker/daemon.json` sudah ada dan VM menjalankan container lain, **jangan ditimpa**. Baca [docs/deployment.md](deployment.md) dulu.

Pasang Docker:

```bash
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh
systemctl enable --now docker
docker compose version
```

## Langkah 4 — Pasang UI

```bash
cd /root
git clone https://github.com/ica4me/govpn-wg-ui.git
cd govpn-wg-ui

cat > .env <<EOF
GOVPN_UI_LISTEN=0.0.0.0:8090
GOVPN_WIREGUARD_VERSION=$(modinfo -F version wireguard)
EOF

docker compose build
docker compose up -d
```

Build pertama kurang lebih **2 menit**.

Cek:

```bash
docker compose ps                      # STATUS: Up  (kolom PORTS memang kosong, itu normal)
ss -ltnp | grep 8090                   # harus *:8090 atau 0.0.0.0:8090
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8090/healthz   # 200
```

## Langkah 5 — Buka port di firewall provider

Di panel cloud (security group/firewall), buka:

| Port | Protokol | Untuk |
|---|---|---|
| `8090` | TCP | UI (hanya saat uji coba) |
| `3478` dan seterusnya | UDP | WireGuard, satu port per site atau group |

**Jangan** buka port `8080`. Itu API backend, sengaja hanya listen di `127.0.0.1`.

## Langkah 6 — Login

Buka `http://IP-VM:8090` di browser.

1. Login dengan **admin / admin**.
2. Kamu langsung dibawa ke halaman **Account** untuk membuat password baru. Username juga boleh diganti.
3. Setelah disimpan, kamu keluar otomatis. Login lagi dengan data baru.

Lupa password? Jalankan di VM:

```bash
/opt/govpn/wg/govpn-wg reset-admin
systemctl restart govpn-wg
```

Login kembali menjadi admin / admin. Tunnel tidak putus.

## Langkah 7 — Mulai pakai

Di UI:

1. **VPN Hub · Sites** → **＋ Create site** untuk tiap kantor/cabang. Isi *Local LAN subnets*, misalnya `192.168.10.0/24`. Klik **↓ Linux install script**, lalu jalankan di gateway site: `sudo bash install-site-<nama>.sh`.
2. Agar dua site saling akses, edit masing-masing site dan pilih site lain di **Allowed destination sites**. Izin berlaku satu arah, jadi atur di kedua site.
3. **VPN Clients** → **＋ Create client**, lalu pilih satu atau beberapa site di **Allowed destination sites**. Satu config client bisa menjangkau semua site yang dipilih. Unduh **↓ WireGuard .conf** (laptop) atau QR (HP).

Config client default **split tunnel**: hanya trafik ke site/LAN yang lewat VPN. Kalau ingin full tunnel, ubah `AllowedIPs` di `.conf` client menjadi `0.0.0.0/0`. Server tidak perlu diubah.

---

## Kalau ada masalah

| Gejala | Penyebab & solusi |
|---|---|
| Halaman UI tidak terbuka dari browser | `ss -ltnp \| grep 8090` menunjukkan `127.0.0.1:8090` → `.env` belum berisi `GOVPN_UI_LISTEN=0.0.0.0:8090`; jalankan `docker compose up -d --force-recreate`. Kalau `curl` di VM sudah 200 → port 8090 belum dibuka di firewall provider |
| `docker ps` tidak menampilkan port | Normal. UI memakai jaringan host |
| Login: *Backend unavailable* | `systemctl status govpn-wg` |
| Site/client tidak online | Port UDP belum dibuka di firewall provider, atau config belum dipasang di perangkat |

Log:

```bash
journalctl -u govpn-wg -n 50 --no-pager      # backend
cd /root/govpn-wg-ui && docker compose logs --tail=50 ui   # UI
```

## Update ke versi terbaru

```bash
cd /root/govpn-wg && git pull && bash scripts/install.sh --yes
cd /root/govpn-wg-ui && git pull && docker compose up -d --build --force-recreate
```

Tunnel, site, client, dan password tidak hilang saat update.
