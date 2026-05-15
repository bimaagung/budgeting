# Daily Reminder — Dynamic Phone List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ganti daftar nomor HP hardcode di n8n dengan endpoint `GET /api/users/phones` yang diambil dari database, sehingga reminder harian otomatis menjangkau semua user terdaftar.

**Architecture:** Backend menambah satu method `GetAllPhones` ke domain `UserRepository` dan postgres implementasinya, lalu handler baru untuk `GET /api/users/phones`. n8n workflow diperbarui agar memanggil endpoint ini di awal flow, lalu Code node memecah array phones menjadi item-item individual untuk di-loop.

**Tech Stack:** Go 1.24, Fiber v2, GORM, PostgreSQL, n8n (self-hosted Docker), WhatsApp Cloud API.

---

## File Structure

| File | Action | Tanggung jawab |
|------|--------|----------------|
| `backend/internal/domain/user.go` | Modify | Tambah `GetAllPhones` ke interface `UserRepository` |
| `backend/internal/platform/postgres/user_repo.go` | Modify | Implementasi `GetAllPhones` |
| `backend/internal/platform/postgres/user_repo_test.go` | Create | Integration test `GetAllPhones` |
| `backend/internal/handler/user_handler.go` | Create | Handler `GET /api/users/phones` |
| `backend/internal/handler/user_handler_test.go` | Create | Unit test handler |
| `backend/internal/handler/router.go` | Modify | Tambah route + parameter `*UserHandler` |
| `backend/cmd/server/main.go` | Modify | Wire `UserHandler` di DI |
| `n8n/workflows/reminder.json` | Modify | Ganti node hardcode → HTTP + Code node |

---

## Task 1: `GetAllPhones` — Domain Interface + Postgres

**Files:**
- Modify: `backend/internal/domain/user.go`
- Modify: `backend/internal/platform/postgres/user_repo.go`
- Create: `backend/internal/platform/postgres/user_repo_test.go`

- [ ] **Step 1: Tulis failing test**

Buat file baru `backend/internal/platform/postgres/user_repo_test.go`:

```go
package postgres_test

import (
	"context"
	"testing"

	"budgeting/internal/domain"
	"budgeting/internal/platform/postgres"

	"github.com/google/uuid"
)

func TestGetAllPhones_ReturnsSeedPhones(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewUserRepository(db)

	phones := []string{"+62811111111", "+62822222222"}
	for _, p := range phones {
		u := domain.User{ID: uuid.New(), Phone: p, Name: "test"}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got, err := repo.GetAllPhones(context.Background())
	if err != nil {
		t.Fatalf("GetAllPhones: %v", err)
	}
	if len(got) < 2 {
		t.Errorf("expected at least 2 phones, got %d: %v", len(got), got)
	}
	phoneSet := make(map[string]bool)
	for _, p := range got {
		phoneSet[p] = true
	}
	for _, want := range phones {
		if !phoneSet[want] {
			t.Errorf("phone %q not found in result %v", want, got)
		}
	}
}

// Empty-table case (nil → []string{}) di-cover oleh TestUserHandler_EmptyPhones_ReturnsEmptyArray
// di handler test. Tidak di-test di sini karena DELETE FROM users bisa melanggar FK dari transactions.
```

- [ ] **Step 2: Jalankan test, pastikan FAIL**

```bash
cd backend
DATABASE_URL_TEST="postgres://postgres:123456@127.0.0.1:5432/budgeting?sslmode=disable" \
  go test ./internal/platform/postgres/ -run TestGetAllPhones -v
```

Expected: FAIL — `repo.GetAllPhones undefined`.

- [ ] **Step 3: Tambah method ke interface domain**

Edit `backend/internal/domain/user.go`, tambah `GetAllPhones` ke interface:

```go
type UserRepository interface {
	FindByPhone(ctx context.Context, phone string) (*User, error)
	Upsert(ctx context.Context, user User) (*User, error)
	GetAllPhones(ctx context.Context) ([]string, error)
}
```

- [ ] **Step 4: Implementasi di postgres**

Edit `backend/internal/platform/postgres/user_repo.go`, tambah method setelah `Upsert`:

```go
func (r *userRepo) GetAllPhones(ctx context.Context) ([]string, error) {
	var phones []string
	err := r.db.WithContext(ctx).Model(&domain.User{}).Pluck("phone", &phones).Error
	if phones == nil {
		phones = []string{}
	}
	return phones, err
}
```

- [ ] **Step 5: Jalankan test, pastikan PASS**

```bash
cd backend
DATABASE_URL_TEST="postgres://postgres:123456@127.0.0.1:5432/budgeting?sslmode=disable" \
  go test ./internal/platform/postgres/ -run TestGetAllPhones -v
```

Expected: `ok budgeting/internal/platform/postgres`.

- [ ] **Step 6: Jalankan seluruh test suite**

```bash
cd backend
DATABASE_URL="postgres://postgres:123456@127.0.0.1:5432/budgeting?sslmode=disable" \
  go test ./...
```

Expected: semua `ok`.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/domain/user.go \
        backend/internal/platform/postgres/user_repo.go \
        backend/internal/platform/postgres/user_repo_test.go
git commit -m "feat(user): add GetAllPhones to UserRepository + postgres impl"
```

---

## Task 2: `UserHandler` — Handler + Unit Test

**Files:**
- Create: `backend/internal/handler/user_handler.go`
- Create: `backend/internal/handler/user_handler_test.go`

- [ ] **Step 1: Tulis failing test**

Buat file `backend/internal/handler/user_handler_test.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type fakePhoneQuerier struct {
	phones []string
	err    error
}

func (f *fakePhoneQuerier) GetAllPhones(ctx context.Context) ([]string, error) {
	return f.phones, f.err
}

func newUserApp(q userPhoneQuerier) *fiber.App {
	app := fiber.New()
	h := &UserHandler{q: q}
	app.Get("/api/users/phones", h.Handle)
	return app
}

func TestUserHandler_ReturnsPhonesAsJSON(t *testing.T) {
	q := &fakePhoneQuerier{phones: []string{"+62811111111", "+62822222222"}}
	app := newUserApp(q)

	req := httptest.NewRequest("GET", "/api/users/phones", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var body struct {
		Phones []string `json:"phones"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Phones) != 2 {
		t.Errorf("phones count: got %d, want 2", len(body.Phones))
	}
	if body.Phones[0] != "+62811111111" {
		t.Errorf("phones[0]: got %q, want %q", body.Phones[0], "+62811111111")
	}
}

func TestUserHandler_EmptyPhones_ReturnsEmptyArray(t *testing.T) {
	q := &fakePhoneQuerier{phones: []string{}}
	app := newUserApp(q)

	req := httptest.NewRequest("GET", "/api/users/phones", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var body struct {
		Phones []string `json:"phones"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Phones == nil {
		t.Error("phones must be [] not null")
	}
}

func TestUserHandler_DBError_Returns500(t *testing.T) {
	q := &fakePhoneQuerier{err: errors.New("db down")}
	app := newUserApp(q)

	req := httptest.NewRequest("GET", "/api/users/phones", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 500 {
		t.Fatalf("status: got %d, want 500", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Jalankan test, pastikan FAIL**

```bash
cd backend
go test ./internal/handler/ -run TestUserHandler -v
```

Expected: FAIL — `userPhoneQuerier undefined`, `UserHandler undefined`.

- [ ] **Step 3: Implementasi handler**

Buat file `backend/internal/handler/user_handler.go`:

```go
package handler

import (
	"context"

	"budgeting/pkg/httputil"

	"github.com/gofiber/fiber/v2"
)

type userPhoneQuerier interface {
	GetAllPhones(ctx context.Context) ([]string, error)
}

type UserHandler struct {
	q userPhoneQuerier
}

func NewUserHandler(q userPhoneQuerier) *UserHandler {
	return &UserHandler{q: q}
}

func (h *UserHandler) Handle(c *fiber.Ctx) error {
	phones, err := h.q.GetAllPhones(c.UserContext())
	if err != nil {
		return httputil.InternalError(c, err)
	}
	if phones == nil {
		phones = []string{}
	}
	return c.JSON(fiber.Map{"phones": phones})
}
```

- [ ] **Step 4: Jalankan test, pastikan PASS**

```bash
cd backend
go test ./internal/handler/ -run TestUserHandler -v
```

Expected: `ok budgeting/internal/handler`.

- [ ] **Step 5: Jalankan seluruh test suite**

```bash
cd backend
DATABASE_URL="postgres://postgres:123456@127.0.0.1:5432/budgeting?sslmode=disable" \
  go test ./...
```

Expected: semua `ok`.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/user_handler.go \
        backend/internal/handler/user_handler_test.go
git commit -m "feat(handler): add GET /api/users/phones handler"
```

---

## Task 3: Wire Handler ke Router + main.go

**Files:**
- Modify: `backend/internal/handler/router.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Update `router.go`**

Edit `backend/internal/handler/router.go`. Ubah signature `NewApp` dan tambah route:

```go
package handler

import (
	"context"

	"budgeting/internal/domain"
	"budgeting/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func NewApp(msg *MessageHandler, reminder *ReminderHandler, goal *GoalHandler, user *UserHandler) *fiber.App {
	app := fiber.New()
	app.Use(logger.New())
	app.Use(recover.New())

	api := app.Group("/api")
	api.Post("/message", msg.Handle)
	api.Get("/reminder/:phone", reminder.Handle)
	api.Post("/goals", goal.SetGoal)
	api.Post("/budgets", goal.SetBudget)
	api.Get("/goals", goal.CheckGoals)
	api.Get("/users/phones", user.Handle)

	return app
}

func resolveUser(ctx context.Context, phone string, uc *usecase.GoalUsecase) (*domain.User, error) {
	return uc.ResolveUser(ctx, phone)
}
```

- [ ] **Step 2: Update `main.go`**

Edit `backend/cmd/server/main.go`. Tambah pembuatan `UserHandler` dan pass ke `NewApp`:

```go
package main

import (
	"log"
	"os"

	"budgeting/internal/handler"
	"budgeting/internal/platform/llm"
	"budgeting/internal/platform/postgres"
	"budgeting/internal/usecase"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	db, err := postgres.NewDB()
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}

	// platform
	provider, err := llm.NewProvider()
	if err != nil {
		log.Fatalf("llm provider: %v", err)
	}
	composer := llm.NewMessageComposer(provider)

	// repositories
	userRepo := postgres.NewUserRepository(db)
	txRepo := postgres.NewTransactionRepository(db)
	goalRepo := postgres.NewSavingsGoalRepository(db)
	budgetRepo := postgres.NewBudgetTargetRepository(db)

	// usecases
	reminderUC := usecase.NewReminderUsecase(txRepo, goalRepo, budgetRepo, userRepo, composer)
	goalUC := usecase.NewGoalUsecase(goalRepo, budgetRepo, userRepo)
	messageUC := usecase.NewMessageUsecase(txRepo, userRepo, composer, reminderUC, goalUC)

	// app
	app := handler.NewApp(
		handler.NewMessageHandler(messageUC),
		handler.NewReminderHandler(reminderUC),
		handler.NewGoalHandler(goalUC),
		handler.NewUserHandler(userRepo),
	)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(app.Listen(":" + port))
}
```

- [ ] **Step 3: Build untuk verifikasi compile**

```bash
cd backend
go build ./...
```

Expected: no output (sukses).

- [ ] **Step 4: Jalankan seluruh test suite**

```bash
cd backend
DATABASE_URL="postgres://postgres:123456@127.0.0.1:5432/budgeting?sslmode=disable" \
  go test ./...
```

Expected: semua `ok`.

- [ ] **Step 5: Smoke test endpoint baru**

Pastikan server berjalan (jalankan di terminal terpisah jika belum):

```bash
cd backend
go run ./cmd/server
```

Lalu test endpoint:

```bash
curl http://localhost:8080/api/users/phones
```

Expected response:
```json
{"phones":["+6281234567890"]}
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/router.go \
        backend/cmd/server/main.go
git commit -m "feat(router): wire GET /api/users/phones into app"
```

---

## Task 4: Update n8n `reminder.json`

**Files:**
- Modify: `n8n/workflows/reminder.json`

- [ ] **Step 1: Ganti isi `reminder.json`**

Tulis ulang seluruh file `n8n/workflows/reminder.json` dengan konten berikut:

```json
{
  "name": "Daily Reminder → WhatsApp",
  "active": false,
  "nodes": [
    {
      "id": "node-cron",
      "name": "Cron 20:00 WIB",
      "type": "n8n-nodes-base.scheduleTrigger",
      "typeVersion": 1,
      "position": [240, 300],
      "parameters": {
        "rule": {
          "interval": [{ "field": "cronExpression", "expression": "0 13 * * *" }]
        }
      },
      "notes": "20:00 WIB = 13:00 UTC. Sesuaikan timezone di n8n Settings jika perlu."
    },
    {
      "id": "node-get-phones",
      "name": "GET Daftar User",
      "type": "n8n-nodes-base.httpRequest",
      "typeVersion": 3,
      "position": [460, 300],
      "parameters": {
        "method": "GET",
        "url": "http://host.docker.internal:8080/api/users/phones",
        "options": {}
      }
    },
    {
      "id": "node-split-phones",
      "name": "Split per Phone",
      "type": "n8n-nodes-base.code",
      "typeVersion": 2,
      "position": [680, 300],
      "parameters": {
        "jsCode": "const phones = $input.item.json.phones;\nreturn phones.map(phone => ({ json: { phone } }));"
      }
    },
    {
      "id": "node-get-reminder",
      "name": "GET Reminder dari Golang",
      "type": "n8n-nodes-base.httpRequest",
      "typeVersion": 3,
      "position": [900, 300],
      "parameters": {
        "method": "GET",
        "url": "=http://host.docker.internal:8080/api/reminder/{{ $json.phone }}",
        "options": {}
      }
    },
    {
      "id": "node-send-wa",
      "name": "Kirim Reminder ke WhatsApp",
      "type": "n8n-nodes-base.httpRequest",
      "typeVersion": 3,
      "position": [1120, 300],
      "parameters": {
        "method": "POST",
        "url": "=https://graph.facebook.com/v19.0/{{ $vars.WA_PHONE_NUMBER_ID }}/messages",
        "sendHeaders": true,
        "headerParameters": {
          "parameters": [
            { "name": "Authorization", "value": "=Bearer {{ $vars.WA_API_TOKEN }}" }
          ]
        },
        "sendBody": true,
        "bodyParameters": {
          "parameters": [
            { "name": "messaging_product", "value": "whatsapp" },
            { "name": "to", "value": "={{ $('Split per Phone').item.json.phone }}" },
            { "name": "type", "value": "text" },
            { "name": "text", "value": "={{ JSON.stringify({ body: $json.message }) }}" }
          ]
        },
        "options": {}
      }
    }
  ],
  "connections": {
    "Cron 20:00 WIB": {
      "main": [[{ "node": "GET Daftar User", "type": "main", "index": 0 }]]
    },
    "GET Daftar User": {
      "main": [[{ "node": "Split per Phone", "type": "main", "index": 0 }]]
    },
    "Split per Phone": {
      "main": [[{ "node": "GET Reminder dari Golang", "type": "main", "index": 0 }]]
    },
    "GET Reminder dari Golang": {
      "main": [[{ "node": "Kirim Reminder ke WhatsApp", "type": "main", "index": 0 }]]
    }
  },
  "settings": {
    "executionOrder": "v1"
  },
  "tags": []
}
```

- [ ] **Step 2: Verifikasi JSON valid**

```bash
cd D:/Developer/RnD/budgeting
node -e "JSON.parse(require('fs').readFileSync('n8n/workflows/reminder.json','utf8')); console.log('valid')"
```

Expected: `valid`.

- [ ] **Step 3: Commit**

```bash
git add n8n/workflows/reminder.json
git commit -m "feat(n8n): replace hardcoded phones with GET /api/users/phones + fix workflow format"
```
