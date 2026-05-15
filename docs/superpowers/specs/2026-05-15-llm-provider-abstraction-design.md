# LLM Provider Abstraction — Design Spec

**Tanggal**: 2026-05-15
**Status**: Approved (siap untuk implementation plan)
**Topik**: Multi-provider LLM abstraction; migrasi default ke Gemini 2.5 Flash (free tier)

---

## 1. Konteks & Motivasi

Backend saat ini hardcoded ke Anthropic Claude API (Haiku 4.5) via `internal/platform/llm/composer.go`. Penggunaan Claude API memerlukan paid plan minimum $5, sementara fase MVP project ini lebih cocok pakai LLM dengan free tier yang generous.

Google Gemini 2.5 Flash menyediakan free tier 1500 request/hari + 1M token/menit via Google AI Studio (tanpa kartu kredit), dengan kualitas Bahasa Indonesia yang excellent untuk use case parsing pesan WA. Ini mencukupi (dan lebih) untuk MVP personal budgeting.

Daripada simply replace Claude dengan Gemini, kita bikin **abstraction multi-provider** supaya:

1. Bisa swap provider via env var (mudah eksperimen Gemini vs Claude vs lainnya nanti)
2. Claude tetap available sebagai opsi (untuk production / future use)
3. Provider baru (Groq, Ollama, OpenAI) bisa ditambah dengan implement 1 interface, tanpa ubah composer/usecase/handler

## 2. Tujuan & Non-Tujuan

### Tujuan
- Interface `LLMProvider` yang generic untuk panggilan LLM (request → response text)
- Implementasi Gemini provider pakai Google official SDK + structured output (responseSchema)
- Implementasi Claude provider yang setara dengan kondisi sekarang
- Factory yang pilih provider via env `LLM_PROVIDER`
- Default provider: `gemini` (untuk MVP free tier)
- Tidak ada perubahan di layer `domain`, `usecase`, `handler`, `postgres`

### Non-Tujuan
- Tidak bikin abstraction untuk multi-model routing (mis. routing parse → Haiku, reminder → Gemini). Single provider per server instance saja.
- Tidak ubah behavior validasi / downgrade existing (`Validate()` + `unknown` fallback tetap dipakai)
- Tidak bikin caching, retry custom, atau circuit breaker — pakai default SDK
- Tidak migrasi prompt content (`prompt.go` tetap apa adanya, kecuali tweak kecil opsional)

## 3. Arsitektur

### Struktur Folder

```
backend/internal/platform/llm/
├── parser.go              (unchanged) — Validate(raw) → ParseResult
├── prompt.go              (unchanged) — parseSystemPrompt, buildPostTxPrompt, dll
├── provider.go            (NEW)       — interface LLMProvider + factory NewProvider()
├── gemini_provider.go     (NEW)       — implementasi pakai google.golang.org/genai
├── claude_provider.go     (NEW)       — implementasi existing, dipindah dari composer.go
└── composer.go            (REFACTOR)  — orchestrator: implement domain.MessageComposer,
                                         delegate ke LLMProvider untuk LLM call
```

### Dependency Direction (Clean Architecture)

```
handler / usecase ──> domain.MessageComposer
                              ▲
                              │ implements
                              │
                         composer (orchestrator)
                              │
                              ▼
                         LLMProvider (interface)
                          ▲          ▲
                          │          │
                          │          │
                   geminiProvider  claudeProvider
                          │          │
                          ▼          ▼
                      Gemini SDK   Anthropic SDK
```

- `composer` depend pada **interface** `LLMProvider`, bukan implementasi
- Provider depend pada SDK vendor masing-masing
- Domain layer **tidak berubah** (`MessageComposer` interface tetap)
- DI di `cmd/server/main.go` yang wire provider concrete ke composer

## 4. Komponen

### 4.1 `provider.go` — Interface & Factory

```go
package llm

import (
    "context"
    "fmt"
    "os"
)

type LLMProvider interface {
    Generate(ctx context.Context, systemPrompt, userPrompt string, expectJSON bool) (string, error)
}

func NewProvider() (LLMProvider, error) {
    name := os.Getenv("LLM_PROVIDER")
    if name == "" {
        name = "gemini"
    }
    switch name {
    case "gemini":
        return newGeminiProvider()
    case "claude":
        return newClaudeProvider()
    default:
        return nil, fmt.Errorf("unknown LLM_PROVIDER: %q (expected: gemini | claude)", name)
    }
}
```

**Kontrak `Generate`:**
- `systemPrompt`: instruksi sistem (peran asisten, format output, dll)
- `userPrompt`: pesan dari user atau context untuk reminder
- `expectJSON`: hint untuk provider supaya enforce JSON output bila support (Gemini pakai responseSchema; Claude tidak butuh — sudah di-instruct via prompt)
- Return: raw text output dari LLM (composer yang Validate jika expectJSON)

### 4.2 `composer.go` — Orchestrator

```go
package llm

import (
    "context"
    "log"

    "budgeting/internal/domain"
)

type composer struct {
    provider LLMProvider
}

func NewMessageComposer(p LLMProvider) domain.MessageComposer {
    return &composer{provider: p}
}

func (c *composer) ParseMessage(ctx context.Context, raw string) (*domain.ParseResult, error) {
    out, err := c.provider.Generate(ctx, parseSystemPrompt, raw, true)
    if err != nil {
        return nil, err
    }
    parsed, vErr := Validate([]byte(out))
    if vErr != nil {
        log.Printf("llm parse validation failed: raw=%s err=%v", out, vErr)
        return &domain.ParseResult{Intent: "unknown", Confidence: 0}, nil
    }
    return &parsed, nil
}

func (c *composer) FormatPostTransaction(ctx context.Context, rc domain.ReminderContext) (string, error) {
    return c.provider.Generate(ctx, reminderSystemPrompt, buildPostTransactionPrompt(rc), false)
}

func (c *composer) FormatDailyReminder(ctx context.Context, rc domain.ReminderContext) (string, error) {
    return c.provider.Generate(ctx, reminderSystemPrompt, buildDailyReminderPrompt(rc), false)
}
```

### 4.3 `gemini_provider.go` — Implementasi Gemini

```go
package llm

import (
    "context"
    "fmt"
    "os"

    "google.golang.org/genai"
)

type geminiProvider struct {
    client *genai.Client
    model  string
}

func newGeminiProvider() (*geminiProvider, error) {
    key := os.Getenv("GEMINI_API_KEY")
    if key == "" {
        return nil, fmt.Errorf("GEMINI_API_KEY is required when LLM_PROVIDER=gemini")
    }
    model := os.Getenv("GEMINI_MODEL")
    if model == "" {
        model = "gemini-2.5-flash"
    }
    client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
        APIKey:  key,
        Backend: genai.BackendGeminiAPI,
    })
    if err != nil {
        return nil, fmt.Errorf("gemini client init: %w", err)
    }
    return &geminiProvider{client: client, model: model}, nil
}

func (g *geminiProvider) Generate(ctx context.Context, system, user string, expectJSON bool) (string, error) {
    cfg := &genai.GenerateContentConfig{
        SystemInstruction: genai.NewContentFromText(system, genai.RoleUser),
        Temperature:       genai.Ptr[float32](0.2),
    }
    if expectJSON {
        cfg.ResponseMIMEType = "application/json"
        cfg.ResponseSchema = parseResponseSchema
    }
    resp, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(user), cfg)
    if err != nil {
        return "", fmt.Errorf("gemini api: %w", err)
    }
    return resp.Text(), nil
}

var parseResponseSchema = &genai.Schema{
    Type: genai.TypeObject,
    Properties: map[string]*genai.Schema{
        "intent": {
            Type: genai.TypeString,
            Enum: []string{
                "expense", "income", "balance", "report",
                "delete_last", "set_goal", "set_budget", "check_goal", "unknown",
            },
        },
        "amount":     {Type: genai.TypeInteger},
        "category":   {Type: genai.TypeString},
        "note":       {Type: genai.TypeString},
        "goal_name":  {Type: genai.TypeString},
        "confidence": {Type: genai.TypeNumber},
    },
    Required: []string{"intent", "amount", "category", "note", "goal_name", "confidence"},
}
```

**Keunggulan struktur ini:**
- Schema strict (Enum di intent) → Gemini guaranteed return valid JSON sesuai schema, drop probabilitas validation error mendekati nol
- Temperature 0.2 → output deterministik untuk parsing
- Schema reusable kalau ada provider lain yang juga support JSON schema

### 4.4 `claude_provider.go` — Implementasi Claude

```go
package llm

import (
    "context"
    "fmt"
    "os"

    "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
)

type claudeProvider struct {
    client anthropic.Client
    model  anthropic.Model
}

func newClaudeProvider() (*claudeProvider, error) {
    key := os.Getenv("ANTHROPIC_API_KEY")
    if key == "" {
        return nil, fmt.Errorf("ANTHROPIC_API_KEY is required when LLM_PROVIDER=claude")
    }
    return &claudeProvider{
        client: anthropic.NewClient(option.WithAPIKey(key)),
        model:  anthropic.ModelClaudeHaiku4_5_20251001,
    }, nil
}

func (c *claudeProvider) Generate(ctx context.Context, system, user string, expectJSON bool) (string, error) {
    maxTok := int64(512)
    if expectJSON {
        maxTok = 256
    }
    msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
        Model:     c.model,
        MaxTokens: maxTok,
        System:    []anthropic.TextBlockParam{{Text: system}},
        Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
    })
    if err != nil {
        return "", fmt.Errorf("claude api: %w", err)
    }
    return msg.Content[0].Text, nil
}
```

Catatan: untuk Claude, `expectJSON` saat ini tidak ada handling khusus — output JSON-only enforced lewat prompt content, dan composer yang Validate. Ini mempertahankan behavior existing 1:1.

### 4.5 `cmd/server/main.go` — DI Wiring

Perubahan minimal (3 baris):

```go
// SEBELUM:
composer := llm.NewMessageComposer()

// SESUDAH:
provider, err := llm.NewProvider()
if err != nil {
    log.Fatalf("llm provider: %v", err)
}
composer := llm.NewMessageComposer(provider)
```

## 5. Konfigurasi

### Environment Variables

| Var | Wajib | Default | Keterangan |
|---|---|---|---|
| `LLM_PROVIDER` | Tidak | `gemini` | Pilih provider: `gemini` \| `claude` |
| `GEMINI_API_KEY` | Wajib jika provider=gemini | — | Dari aistudio.google.com, free no credit card |
| `GEMINI_MODEL` | Tidak | `gemini-2.5-flash` | Override model name bila perlu |
| `ANTHROPIC_API_KEY` | Wajib jika provider=claude | — | Existing, tidak dihapus |

### .env.example

File `.env.example` di-update dengan tiga var baru (`LLM_PROVIDER`, `GEMINI_API_KEY`, `GEMINI_MODEL`) lengkap dengan komentar.

## 6. Error Handling

| Skenario | Behavior |
|---|---|
| `LLM_PROVIDER` tidak dikenal | `main.go` log.Fatalf saat startup |
| API key kosong untuk provider aktif | `main.go` log.Fatalf saat startup |
| Gemini SDK gagal init | `main.go` log.Fatalf saat startup |
| API call error (network, rate limit, 5xx) | Bubble up `error` → usecase translate ke reply `error` (existing) |
| Output JSON malformed (Claude only — Gemini guaranteed via schema) | `Validate()` fail → log raw + downgrade ke `{intent:"unknown", confidence:0}` (existing) |
| Output text kosong (rare; Gemini safety filter) | Return `""` → usecase existing handle sebagai empty reply |

Prinsip: **error handling di composer & layer atas tidak berubah**. Provider hanya translate API error menjadi `error`.

## 7. Testing

| Test | File | Tujuan |
|---|---|---|
| Validate existing | `parser_test.go`, `parser_validate_test.go` | Tetap jalan tanpa perubahan |
| Composer with mock provider | `composer_test.go` (NEW) | Verifikasi orchestration: prompt routing, Validate dipanggil, malformed → downgrade |
| Gemini provider integration | `gemini_provider_test.go` (NEW, skip jika no API key) | Smoke test real API dengan sample pesan ID |
| Provider factory | `provider_test.go` (NEW) | Env routing + missing key + unknown name |

Mock provider untuk composer test:

```go
type stubProvider struct {
    out string
    err error
}
func (s *stubProvider) Generate(ctx context.Context, sys, user string, expectJSON bool) (string, error) {
    return s.out, s.err
}
```

Semua unit test composer bisa jalan **tanpa API key real**; integration test eksplisit skip jika `GEMINI_API_KEY` kosong.

## 8. Dependency Changes

| Aksi | Package |
|---|---|
| Tambah | `google.golang.org/genai` |
| Tetap | `github.com/anthropics/anthropic-sdk-go` |
| Tidak berubah | Semua dependency lain (gofiber, pgx, uuid, godotenv, dll) |

## 9. Migration Order (Implementation Plan Hint)

Ini sebagai input untuk writing-plans nanti, bukan langkah final:

1. `go get google.golang.org/genai`
2. Bikin `provider.go` (interface + factory)
3. Bikin `gemini_provider.go` (full impl)
4. Bikin `claude_provider.go` (pindahkan dari composer.go lama)
5. Refactor `composer.go` jadi orchestrator
6. Update `cmd/server/main.go` wiring
7. Update `.env.example`
8. Update `CLAUDE.md` (tabel tech stack: ganti "Claude API" → "LLM (Gemini default, Claude opsional)" + section konfigurasi)
9. Tambah tests
10. Run `go test ./...`
11. Smoke test manual: kirim pesan via n8n

## 10. Risiko & Mitigasi

| Risiko | Probabilitas | Dampak | Mitigasi |
|---|---|---|---|
| Gemini free tier rate limit kena (1500 req/hari) | Rendah untuk MVP personal | Sedang | Set fallback ke Claude via env swap kalau perlu |
| Kualitas parsing Gemini berbeda dengan Claude | Sedang | Rendah | `Validate()` + downgrade ke `unknown` masih jaga safety; confidence threshold tetap berlaku |
| Breaking change di SDK `google.golang.org/genai` (relatif baru) | Rendah | Rendah | Pin versi di go.mod; abstraction layer batasi blast radius |
| API key bocor di git | Rendah | Tinggi | `.env` sudah di `.gitignore`; hanya `.env.example` di-commit |

## 11. Roadmap Setelah Implementasi

- Tambah provider baru (Groq Llama 3.3, OpenAI, Ollama lokal) hanya butuh:
  1. Implement `LLMProvider` interface (1 method)
  2. Tambah case di factory `NewProvider()`
  3. Tambah env var documentation
- Eksperimen split provider per fungsi (parse vs reminder) kalau perlu — tinggal extend interface atau bikin composer dengan 2 provider.
