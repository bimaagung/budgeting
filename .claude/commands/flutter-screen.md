# Skill: flutter-screen

Scaffold Flutter screen baru untuk aplikasi budgeting Android.

## Tugas

Buat screen Flutter: **$ARGUMENTS**

## Langkah Implementasi

### 1. Buat feature folder
`mobile/lib/features/<nama_feature>/`
- `<nama>_screen.dart` — widget utama (StatelessWidget atau ConsumerWidget)
- `<nama>_controller.dart` — state management & panggil API (gunakan Riverpod jika sudah dipakai project)
- `<nama>_model.dart` — data class jika screen punya model spesifik

### 2. API layer (`mobile/lib/core/api/`)
- Tambahkan method baru di file API client yang relevan
- Gunakan `http` atau `dio` sesuai yang sudah dipakai project
- Parse response JSON ke model menggunakan `fromJson`

### 3. Navigasi
- Daftarkan route di file routing utama project
- Tambahkan navigasi dari screen yang relevan

## Konvensi Flutter Project Ini
- Flutter hanya **read-only**: tidak ada form input transaksi di app (semua lewat WA)
- Format currency: gunakan `NumberFormat.currency(locale: 'id_ID', symbol: 'Rp ')` dari package `intl`
- Format tanggal: locale `id_ID`
- Pisahkan UI widget dan logic controller — jangan taruh HTTP call di dalam widget build
