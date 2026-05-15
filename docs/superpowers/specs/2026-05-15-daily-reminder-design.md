# Daily Reminder — Dynamic Phone List Design

## Goal

Mengaktifkan daily reminder via WhatsApp dengan daftar nomor HP yang diambil secara dinamis dari database, bukan hardcode di n8n.

## Arsitektur

Backend menambah satu endpoint `GET /api/users/phones` yang mengembalikan semua nomor terdaftar. n8n memanggil endpoint ini di awal flow reminder, lalu loop per nomor untuk generate dan kirim pesan reminder.

URL backend dari n8n menggunakan `host.docker.internal:8080` karena n8n berjalan di Docker container dengan backend Go di host yang sama.

## Komponen yang Berubah

### 1. `internal/domain/user.go`

Tambah method ke interface `UserRepository`:

```go
GetAllPhones(ctx context.Context) ([]string, error)
```

### 2. `internal/platform/postgres/user_repo.go`

Implementasi `GetAllPhones`:

```go
func (r *userRepo) GetAllPhones(ctx context.Context) ([]string, error) {
    var phones []string
    err := r.db.WithContext(ctx).Model(&domain.User{}).
        Pluck("phone", &phones).Error
    return phones, err
}
```

Query: `SELECT phone FROM users` — ambil semua nomor tanpa filter (semua user dianggap aktif).

### 3. `internal/handler/user_handler.go` (file baru)

Handler untuk endpoint baru:

```go
GET /api/users/phones
→ 200 {"phones": ["+6281234567890", "+628..."]}
→ 500 jika DB error
```

Interface `userPhoneQuerier` di-define lokal di file ini (dependency inversion, testable tanpa usecase layer).

### 4. `internal/handler/router.go`

Tambah route:

```go
api.Get("/users/phones", userHandler.Handle)
```

`NewApp` menerima `*UserHandler` sebagai parameter tambahan. `main.go` di-update untuk wire `UserHandler`.

### 5. `n8n/workflows/reminder.json`

Flow baru:

```
Cron (20:00) → GET /api/users/phones → Split per phone → GET /api/reminder/:phone → Kirim WA
```

Perubahan spesifik:
- Node "Daftar Nomor User" (type: `n8n-nodes-base.set`, hardcode) **dihapus**
- Diganti node "GET Daftar User" (type: `n8n-nodes-base.httpRequest`) ke `http://host.docker.internal:8080/api/users/phones`
- Node "Loop per User" (`splitInBatches`) menerima array `phones` dari response HTTP
- URL node "GET Reminder dari Golang" diupdate menggunakan item dari split: `http://host.docker.internal:8080/api/reminder/{{ $json }}`
- Format JSON diperbaiki: setiap node memiliki `id`, `typeVersion`, `position`

## Data Flow

```
n8n Cron (20:00 WIB)
  └→ GET host.docker.internal:8080/api/users/phones
       └→ {"phones": ["+628A", "+628B"]}
            └→ splitInBatches (batchSize: 1)
                 └→ GET host.docker.internal:8080/api/reminder/+628A
                      └→ {"message": "📅 Ringkasan hari ini..."}
                           └→ POST graph.facebook.com WA API (kirim ke +628A)
```

## Testing

### Unit tests

**`user_repo_test.go`** (integration, pakai `DATABASE_URL_TEST`):
- `TestGetAllPhones_ReturnsAllPhones` — insert 2 user, assert phones returned
- `TestGetAllPhones_EmptyTable` — assert returns empty slice bukan error

**`user_handler_test.go`** (unit, pakai fake querier):
- `TestUserHandler_ReturnsPhonesAsJSON` — assert response body `{"phones":[...]}`
- `TestUserHandler_DBError_Returns500` — querier return error → HTTP 500

### Smoke test manual

```bash
# Backend harus jalan
curl http://localhost:8080/api/users/phones
# → {"phones":["+6281234567890"]}

# Reminder endpoint tetap berjalan
curl http://localhost:8080/api/reminder/+6281234567890
# → {"message":"...reminder text..."}
```

## Keputusan Desain

- **Tidak ada auth** pada `/api/users/phones` — endpoint internal, hanya diakses n8n di jaringan yang sama. Konsisten dengan `/api/reminder/:phone` yang juga tidak ada auth.
- **Tidak ada filter aktif/nonaktif** — semua user di tabel `users` dianggap aktif. Jika nanti perlu filter, cukup tambah kolom `is_active` dan update query.
- **`userPhoneQuerier` interface di handler** — handler tidak depend pada `UserRepository` domain secara langsung, cukup interface minimal. Ini konsisten dengan pola `messageUsecase` interface di `message_handler.go`.
