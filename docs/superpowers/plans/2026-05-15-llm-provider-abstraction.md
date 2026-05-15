# LLM Provider Abstraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `internal/platform/llm/` jadi multi-provider abstraction dengan interface `LLMProvider`. Migrasi default provider ke Google Gemini 2.5 Flash (free tier) sambil mempertahankan Claude Haiku sebagai opsi swap-able via env var.

**Architecture:** Tambah interface `LLMProvider` (1 method: `Generate`). Composer existing direfactor jadi orchestrator yang depend pada interface. Dua implementasi: `geminiProvider` (Google `google.golang.org/genai` SDK + `responseSchema` untuk structured output) dan `claudeProvider` (existing Anthropic SDK, dipindah dari `composer.go`). Factory `NewProvider()` pilih implementasi via env `LLM_PROVIDER`. Domain / usecase / handler / repo **tidak berubah** — interface `domain.MessageComposer` tetap, hanya implementasi di `composer.go` yang direfactor.

**Tech Stack:** Go 1.23, `google.golang.org/genai` (baru), `github.com/anthropics/anthropic-sdk-go` (existing), Fiber v2, stdlib `testing` + hand-rolled fakes (no testify).

**Spec:** `docs/superpowers/specs/2026-05-15-llm-provider-abstraction-design.md`

**Working directory:** Semua perintah shell mengasumsikan `cwd = D:\Developer\RnD\budgeting\backend` kecuali path dimulai dengan `docs/`, `openspec/`, atau root files (`CLAUDE.md`).

---

## Phase 0 — Pre-flight

### Task 0: Verifikasi baseline

**Files:** none (verification only)

- [ ] **Step 1: Konfirmasi working tree bersih**

Run (dari repo root): `git status`
Expected: hanya menampilkan modifikasi yang sudah ada sebelumnya (`.claude/settings.local.json`, `n8n/workflows/wa-inbound.json`) dan `?? docs/superpowers/plans/2026-05-15-llm-provider-abstraction.md`. Tidak ada perubahan di `backend/`.

- [ ] **Step 2: Run existing tests**

Run (dari `backend/`): `go test ./... -count=1`
Expected: PASS untuk semua package. `parser_test.go` (live API test) akan skip kalau `ANTHROPIC_API_KEY` tidak diset (lihat `parser_test.go:14`).

- [ ] **Step 3: Verifikasi Go version**

Run (dari `backend/`): `go version`
Expected: Go 1.23.x atau lebih tinggi (genai SDK butuh Go 1.21+).

---

## Phase 1 — Setup Dependency

### Task 1: Tambah Gemini SDK ke go.mod

**Files:**
- Modify: `backend/go.mod`
- Modify: `backend/go.sum` (auto-generated)

- [ ] **Step 1: Add `google.golang.org/genai`**

Run (dari `backend/`): `go get google.golang.org/genai@latest`
Expected: SDK ditambahkan ke `go.mod` di blok `require`. Jika ada output `go: downloading ...`, itu normal.

- [ ] **Step 2: Tidy modules**

Run: `go mod tidy`
Expected: tidak ada error. `go.sum` di-update.

- [ ] **Step 3: Verifikasi compile**

Run: `go build ./...`
Expected: build sukses (belum ada kode yang pakai genai, jadi build harus tetap clean).

- [ ] **Step 4: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "$(cat <<'EOF'
chore(deps): add google.golang.org/genai for Gemini provider

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2 — Definisi Interface

### Task 2: Buat `LLMProvider` interface

**Files:**
- Create: `backend/internal/platform/llm/provider.go`

- [ ] **Step 1: Tulis interface saja (tanpa factory dulu)**

Buat file `backend/internal/platform/llm/provider.go`:

```go
package llm

import "context"

// LLMProvider — kontrak adapter ke vendor LLM (Gemini, Claude, dll).
// Composer pakai ini untuk LLM call mentah; prompt building & JSON validation
// dilakukan di layer composer (DRY antar provider).
//
// Generate menerima system prompt + user prompt + flag expectJSON.
// expectJSON adalah hint untuk provider supaya enforce JSON output bila support
// (Gemini pakai responseSchema; Claude tidak butuh — sudah di-instruct via prompt).
// Composer yang akan Validate() raw output bila perlu.
type LLMProvider interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string, expectJSON bool) (string, error)
}
```

- [ ] **Step 2: Verifikasi compile**

Run: `go build ./...`
Expected: build sukses. Interface unused (belum ada implementor) tidak mengganggu compile.

- [ ] **Step 3: Verifikasi tests masih hijau**

Run: `go test ./... -count=1`
Expected: PASS (tidak ada perubahan behavior).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/platform/llm/provider.go
git commit -m "$(cat <<'EOF'
feat(llm): define LLMProvider interface (stub, no factory yet)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3 — Claude Provider (Extract Existing Logic)

### Task 3: Pindahkan logic Claude ke `claude_provider.go`

**Files:**
- Create: `backend/internal/platform/llm/claude_provider.go`
- Test: `backend/internal/platform/llm/claude_provider_test.go`

- [ ] **Step 1: Tulis test untuk constructor (RED)**

Buat file `backend/internal/platform/llm/claude_provider_test.go`:

```go
package llm

import (
	"os"
	"testing"
)

func TestNewClaudeProvider_RequiresAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := newClaudeProvider()
	if err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY is empty, got nil")
	}
}

func TestNewClaudeProvider_WithAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-dummy")
	p, err := newClaudeProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}

// Hindari unused-import error jika import os hanya untuk t.Setenv (which uses os internally)
var _ = os.Getenv
```

- [ ] **Step 2: Run test, harus FAIL**

Run: `go test ./internal/platform/llm/ -run TestNewClaudeProvider -v -count=1`
Expected: FAIL dengan `undefined: newClaudeProvider`.

- [ ] **Step 3: Buat `claude_provider.go` (GREEN)**

Buat file `backend/internal/platform/llm/claude_provider.go`:

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
	// expectJSON tidak butuh special handling: output JSON-only sudah di-enforce
	// via prompt content, dan composer yang Validate() output.
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

- [ ] **Step 4: Run test, harus PASS**

Run: `go test ./internal/platform/llm/ -run TestNewClaudeProvider -v -count=1`
Expected: PASS untuk kedua test (`TestNewClaudeProvider_RequiresAPIKey`, `TestNewClaudeProvider_WithAPIKey`).

- [ ] **Step 5: Verifikasi semua test masih hijau**

Run: `go test ./... -count=1`
Expected: PASS (composer.go lama masih ada dan masih kerja paralel, belum di-refactor).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/platform/llm/claude_provider.go backend/internal/platform/llm/claude_provider_test.go
git commit -m "$(cat <<'EOF'
feat(llm): extract Claude logic into claude_provider.go

Implements LLMProvider via Anthropic SDK (Haiku 4.5). Constructor
fails fast when ANTHROPIC_API_KEY is missing. Existing composer.go
keeps working in parallel until Task 6 refactor.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4 — Gemini Provider

### Task 4: Buat `gemini_provider.go`

**Files:**
- Create: `backend/internal/platform/llm/gemini_provider.go`
- Test: `backend/internal/platform/llm/gemini_provider_test.go`

- [ ] **Step 1: Tulis test untuk constructor (RED)**

Buat file `backend/internal/platform/llm/gemini_provider_test.go`:

```go
package llm

import (
	"context"
	"os"
	"testing"
)

func TestNewGeminiProvider_RequiresAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	_, err := newGeminiProvider()
	if err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is empty, got nil")
	}
}

func TestNewGeminiProvider_WithAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "dummy-key")
	t.Setenv("GEMINI_MODEL", "")
	p, err := newGeminiProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
	if p.model != "gemini-2.5-flash" {
		t.Errorf("expected default model 'gemini-2.5-flash', got %q", p.model)
	}
}

func TestNewGeminiProvider_CustomModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "dummy-key")
	t.Setenv("GEMINI_MODEL", "gemini-2.5-flash-lite")
	p, err := newGeminiProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.model != "gemini-2.5-flash-lite" {
		t.Errorf("expected model 'gemini-2.5-flash-lite', got %q", p.model)
	}
}

// TestGeminiProvider_GenerateLive — smoke test real API, skip kalau no key.
func TestGeminiProvider_GenerateLive(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" || key == "dummy-key" {
		t.Skip("GEMINI_API_KEY not set; live test skipped")
	}
	p, err := newGeminiProvider()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	out, err := p.Generate(context.Background(),
		"Kamu balas pesan dengan satu kata dalam Bahasa Indonesia.",
		"Sapa saya!",
		false)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	t.Logf("gemini response: %q", out)
}
```

- [ ] **Step 2: Run test, harus FAIL**

Run: `go test ./internal/platform/llm/ -run TestNewGeminiProvider -v -count=1`
Expected: FAIL dengan `undefined: newGeminiProvider`.

- [ ] **Step 3: Buat `gemini_provider.go` (GREEN)**

Buat file `backend/internal/platform/llm/gemini_provider.go`:

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

// parseResponseSchema — strict JSON schema untuk ParseResult.
// Gemini ENFORCE output sesuai schema ini, sehingga validation hampir tidak pernah fail.
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

- [ ] **Step 4: Run test, harus PASS**

Run: `go test ./internal/platform/llm/ -run TestNewGeminiProvider -v -count=1`
Expected: PASS untuk ketiga test (`RequiresAPIKey`, `WithAPIKey`, `CustomModel`). `GenerateLive` akan skip tanpa key.

- [ ] **Step 5: (Opsional) Run live test kalau ada GEMINI_API_KEY**

Run: `GEMINI_API_KEY=<real-key> go test ./internal/platform/llm/ -run TestGeminiProvider_GenerateLive -v -count=1`
Expected: PASS dengan log response Gemini. Skip kalau tidak punya key — ini opsional di tahap ini.

- [ ] **Step 6: Verifikasi semua test masih hijau**

Run: `go test ./... -count=1`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/platform/llm/gemini_provider.go backend/internal/platform/llm/gemini_provider_test.go
git commit -m "$(cat <<'EOF'
feat(llm): add Gemini provider with structured output

Implements LLMProvider via google.golang.org/genai. Uses responseSchema
with intent enum to guarantee valid JSON for ParseMessage calls.
Default model: gemini-2.5-flash. Override via GEMINI_MODEL env.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5 — Factory

### Task 5: Tambah `NewProvider()` factory di `provider.go`

**Files:**
- Modify: `backend/internal/platform/llm/provider.go`
- Test: `backend/internal/platform/llm/provider_test.go`

- [ ] **Step 1: Tulis test untuk factory (RED)**

Buat file `backend/internal/platform/llm/provider_test.go`:

```go
package llm

import (
	"strings"
	"testing"
)

func TestNewProvider_DefaultsToGemini(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "")
	t.Setenv("GEMINI_API_KEY", "dummy")
	t.Setenv("GEMINI_MODEL", "")
	p, err := NewProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*geminiProvider); !ok {
		t.Errorf("expected *geminiProvider, got %T", p)
	}
}

func TestNewProvider_SelectsClaude(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	p, err := NewProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*claudeProvider); !ok {
		t.Errorf("expected *claudeProvider, got %T", p)
	}
}

func TestNewProvider_UnknownProvider(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "openai")
	_, err := NewProvider()
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown LLM_PROVIDER") {
		t.Errorf("error message unexpected: %v", err)
	}
}

func TestNewProvider_GeminiMissingKey(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "")
	_, err := NewProvider()
	if err == nil {
		t.Fatal("expected error for missing GEMINI_API_KEY")
	}
}

func TestNewProvider_ClaudeMissingKey(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, err := NewProvider()
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY")
	}
}
```

- [ ] **Step 2: Run test, harus FAIL**

Run: `go test ./internal/platform/llm/ -run TestNewProvider -v -count=1`
Expected: FAIL dengan `undefined: NewProvider`.

- [ ] **Step 3: Tambah factory ke `provider.go` (GREEN)**

Update file `backend/internal/platform/llm/provider.go` jadi:

```go
package llm

import (
	"context"
	"fmt"
	"os"
)

// LLMProvider — kontrak adapter ke vendor LLM (Gemini, Claude, dll).
// Composer pakai ini untuk LLM call mentah; prompt building & JSON validation
// dilakukan di layer composer (DRY antar provider).
//
// Generate menerima system prompt + user prompt + flag expectJSON.
// expectJSON adalah hint untuk provider supaya enforce JSON output bila support
// (Gemini pakai responseSchema; Claude tidak butuh — sudah di-instruct via prompt).
// Composer yang akan Validate() raw output bila perlu.
type LLMProvider interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string, expectJSON bool) (string, error)
}

// NewProvider memilih implementasi LLM berdasarkan env LLM_PROVIDER.
// Default: "gemini". Fail fast kalau API key untuk provider aktif tidak ada
// atau nama provider tidak dikenal.
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

- [ ] **Step 4: Run test, harus PASS**

Run: `go test ./internal/platform/llm/ -run TestNewProvider -v -count=1`
Expected: PASS untuk kelima test factory.

- [ ] **Step 5: Verifikasi semua test masih hijau**

Run: `go test ./... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/platform/llm/provider.go backend/internal/platform/llm/provider_test.go
git commit -m "$(cat <<'EOF'
feat(llm): add NewProvider() factory selecting provider via env

Routes LLM_PROVIDER=gemini|claude to the respective constructor.
Defaults to "gemini" when env is empty. Returns error for unknown
provider names or missing API keys.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 6 — Refactor Composer + Wire Factory

### Task 6: Refactor `composer.go` jadi orchestrator + update wiring

> **Catatan:** Task ini menyentuh `composer.go`, `cmd/server/main.go`, dan `parser_test.go` sekaligus karena perubahan signature `NewMessageComposer` membuat ketiganya saling terikat. Pisah jadi commit terpisah akan menghasilkan intermediate state yang tidak compile.

**Files:**
- Modify: `backend/internal/platform/llm/composer.go` (rewrite isi)
- Modify: `backend/cmd/server/main.go` (3 baris)
- Modify: `backend/internal/platform/llm/parser_test.go` (1 baris signature)
- Test: `backend/internal/platform/llm/composer_test.go`

- [ ] **Step 1: Tulis composer test dengan stub provider (RED)**

Buat file `backend/internal/platform/llm/composer_test.go`:

```go
package llm

import (
	"context"
	"errors"
	"testing"

	"budgeting/internal/domain"
)

type stubProvider struct {
	out          string
	err          error
	gotSystem    string
	gotUser      string
	gotExpectJSON bool
	calls        int
}

func (s *stubProvider) Generate(ctx context.Context, sys, user string, expectJSON bool) (string, error) {
	s.calls++
	s.gotSystem = sys
	s.gotUser = user
	s.gotExpectJSON = expectJSON
	return s.out, s.err
}

func TestComposer_ParseMessage_ValidJSON(t *testing.T) {
	stub := &stubProvider{
		out: `{"intent":"expense","amount":45000,"category":"Makan & Minum","note":"makan siang","goal_name":"","confidence":0.95}`,
	}
	c := NewMessageComposer(stub)
	got, err := c.ParseMessage(context.Background(), "makan siang 45rb")
	if err != nil {
		t.Fatalf("ParseMessage err: %v", err)
	}
	if got.Intent != "expense" {
		t.Errorf("intent: got %q, want %q", got.Intent, "expense")
	}
	if got.Amount != 45000 {
		t.Errorf("amount: got %d, want %d", got.Amount, 45000)
	}
	if !stub.gotExpectJSON {
		t.Errorf("expected expectJSON=true for ParseMessage call")
	}
	if stub.gotSystem != parseSystemPrompt {
		t.Errorf("expected parseSystemPrompt to be used")
	}
}

func TestComposer_ParseMessage_MalformedDowngrades(t *testing.T) {
	stub := &stubProvider{out: `not valid json`}
	c := NewMessageComposer(stub)
	got, err := c.ParseMessage(context.Background(), "garbage")
	if err != nil {
		t.Fatalf("ParseMessage err: %v", err)
	}
	if got.Intent != "unknown" {
		t.Errorf("intent: got %q, want %q (downgrade on malformed)", got.Intent, "unknown")
	}
	if got.Confidence != 0 {
		t.Errorf("confidence: got %f, want 0", got.Confidence)
	}
}

func TestComposer_ParseMessage_ProviderError(t *testing.T) {
	stub := &stubProvider{err: errors.New("api down")}
	c := NewMessageComposer(stub)
	_, err := c.ParseMessage(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected provider error to propagate")
	}
}

func TestComposer_FormatPostTransaction_UsesReminderPrompt(t *testing.T) {
	stub := &stubProvider{out: "✅ Tercatat: Rp 45.000"}
	c := NewMessageComposer(stub)
	rc := domain.ReminderContext{Balance: 100000, LastCategory: "Makan & Minum", LastAmount: 45000}
	out, err := c.FormatPostTransaction(context.Background(), rc)
	if err != nil {
		t.Fatalf("FormatPostTransaction err: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if stub.gotSystem != reminderSystemPrompt {
		t.Errorf("expected reminderSystemPrompt to be used")
	}
	if stub.gotExpectJSON {
		t.Errorf("expectJSON should be false for reminder formatting")
	}
}

func TestComposer_FormatDailyReminder_UsesReminderPrompt(t *testing.T) {
	stub := &stubProvider{out: "📅 Ringkasan hari ini..."}
	c := NewMessageComposer(stub)
	rc := domain.ReminderContext{Balance: 100000}
	out, err := c.FormatDailyReminder(context.Background(), rc)
	if err != nil {
		t.Fatalf("FormatDailyReminder err: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if stub.gotSystem != reminderSystemPrompt {
		t.Errorf("expected reminderSystemPrompt to be used")
	}
}
```

- [ ] **Step 2: Run test, harus FAIL**

Run: `go test ./internal/platform/llm/ -run TestComposer -v -count=1`
Expected: FAIL dengan compile error — `NewMessageComposer` saat ini punya signature `()` (no arg), tapi test memanggil dengan `(stub)`. Compile error ini valid dan ditarget berikutnya.

- [ ] **Step 3: Rewrite `composer.go` (GREEN)**

Ganti seluruh isi file `backend/internal/platform/llm/composer.go` dengan:

```go
package llm

import (
	"context"
	"log"

	"budgeting/internal/domain"
)

// composer adalah orchestrator yang implement domain.MessageComposer.
// Ia delegate panggilan LLM ke LLMProvider (yang concrete impl-nya dipilih
// di main.go via NewProvider()). Prompt building & JSON validation
// dilakukan di sini supaya tidak duplikat antar provider.
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

- [ ] **Step 4: Update `cmd/server/main.go` (wire factory)**

Edit `backend/cmd/server/main.go` — ganti baris `composer := llm.NewMessageComposer()` (sekitar baris 24) jadi:

```go
	// platform
	provider, err := llm.NewProvider()
	if err != nil {
		log.Fatalf("llm provider: %v", err)
	}
	composer := llm.NewMessageComposer(provider)
```

Catatan: `err` di-shadow di blok `db, err := postgres.NewDB()` sebelumnya. Pastikan reassign-nya `provider, err := llm.NewProvider()` — Go akan recognize ini sebagai new var `provider` + reuse `err`. Kalau compiler complain "no new variables on left side of :=", ganti jadi:

```go
	provider, perr := llm.NewProvider()
	if perr != nil {
		log.Fatalf("llm provider: %v", perr)
	}
	composer := llm.NewMessageComposer(provider)
```

- [ ] **Step 5: Update `parser_test.go` (signature change)**

File existing `backend/internal/platform/llm/parser_test.go` baris 13-19 saat ini:

```go
func TestParseMessage_LiveAPI(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set; live LLM test skipped")
	}

	composer := llm.NewMessageComposer()
	ctx := context.Background()
```

Ganti menjadi (preserve `ctx := context.Background()` line):

```go
func TestParseMessage_LiveAPI(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set; live LLM test skipped")
	}

	t.Setenv("LLM_PROVIDER", "claude")
	provider, err := llm.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	composer := llm.NewMessageComposer(provider)
	ctx := context.Background()
```

Bagian bawah test (loop dengan `tests := []struct{...}`) tetap apa adanya tidak diubah.

(Test ini sekarang explicitly minta provider Claude untuk preserve behavior. Test serupa untuk Gemini bisa ditambah di Task 8 kalau mau, tapi opsional.)

- [ ] **Step 6: Verifikasi compile**

Run: `go build ./...`
Expected: build sukses.

- [ ] **Step 7: Run semua test**

Run: `go test ./... -count=1`
Expected: PASS untuk semua. Test composer pakai stub (tidak butuh API key). Test live di `parser_test.go` skip kalau `ANTHROPIC_API_KEY` kosong.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/platform/llm/composer.go backend/internal/platform/llm/composer_test.go backend/internal/platform/llm/parser_test.go backend/cmd/server/main.go
git commit -m "$(cat <<'EOF'
refactor(llm): composer delegates to LLMProvider; wire factory in main

composer.go is now a thin orchestrator that owns prompt routing and
JSON validation. Concrete provider (Gemini or Claude) is injected via
the factory NewProvider() in cmd/server/main.go.

parser_test.go live-API test pinned to claude provider for now.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 7 — Configuration & Documentation

### Task 7: Buat `.env.example` + update `CLAUDE.md`

**Files:**
- Create: `backend/.env.example`
- Modify: `CLAUDE.md` (root)

- [ ] **Step 1: Buat `.env.example`**

Buat file `backend/.env.example` dengan isi:

```bash
# ===== Database =====
DATABASE_URL=postgres://postgres:CHANGE_ME@127.0.0.1:5432/budgeting?sslmode=disable

# ===== Server =====
SERVER_PORT=8080

# ===== LLM Provider =====
# Pilih: "gemini" (default, free tier) atau "claude" (paid)
LLM_PROVIDER=gemini

# --- Gemini (wajib jika LLM_PROVIDER=gemini) ---
# Dapatkan dari https://aistudio.google.com (free, no credit card)
GEMINI_API_KEY=
# Optional: override model (default: gemini-2.5-flash)
# GEMINI_MODEL=gemini-2.5-flash

# --- Claude (wajib jika LLM_PROVIDER=claude) ---
# Dapatkan dari https://console.anthropic.com
ANTHROPIC_API_KEY=
```

- [ ] **Step 2: Update `CLAUDE.md` — tabel Tech Stack**

Cari di `CLAUDE.md` baris `| LLM Understanding | Claude API (Anthropic) | Parse & kategorisasi pesan bebas dari user |` dan ganti jadi:

```markdown
| LLM Understanding | Gemini 2.5 Flash (default) / Claude Haiku (opsional) | Parse & kategorisasi pesan bebas dari user. Provider di-pilih via env `LLM_PROVIDER`. |
```

- [ ] **Step 3: Update `CLAUDE.md` — section "Prinsip Arsitektur"**

Cari bullet `- **LLM Understanding ada di backend (\`internal/platform/llm\`)**: ...` dan tambahkan kalimat ekor:

```markdown
- **LLM Understanding ada di backend (`internal/platform/llm`)**: lebih mudah di-test, retry, dan diganti model tanpa ubah n8n. Validator (`parser.go`) ada di layer ini, bukan di pkg. Provider abstraction (`LLMProvider`) memungkinkan swap antar vendor (Gemini, Claude, dll) via env `LLM_PROVIDER` tanpa ubah usecase / handler.
```

- [ ] **Step 4: Update `CLAUDE.md` — struktur direktori target**

Cari block tree di `CLAUDE.md` yang menggambarkan `platform/llm/` dan update jadi:

```
│       └── llm/
│           ├── provider.go               # interface LLMProvider + factory NewProvider()
│           ├── gemini_provider.go        # implementasi Gemini 2.5 Flash (default)
│           ├── claude_provider.go        # implementasi Claude Haiku (opsional)
│           ├── composer.go               # orchestrator: implements MessageComposer
│           ├── prompt.go                 # system prompt builder + kategori + intents
│           ├── parser.go                 # Validate(raw) → ParseResult
│           └── *_test.go                 # composer + provider + parser tests
```

- [ ] **Step 5: Verifikasi tidak ada test yang rusak**

Run: `go test ./... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/.env.example CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: add .env.example and update CLAUDE.md for LLM abstraction

.env.example documents LLM_PROVIDER, GEMINI_API_KEY, GEMINI_MODEL,
and ANTHROPIC_API_KEY. CLAUDE.md tech stack table + arch principles
+ folder tree reflect the new provider layer.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 8 — Verifikasi Final

### Task 8: Final smoke test

**Files:** none (verifikasi only)

- [ ] **Step 1: Verifikasi build**

Run (dari `backend/`): `go build ./...`
Expected: build sukses, exit 0.

- [ ] **Step 2: Verifikasi test suite penuh**

Run: `go test ./... -count=1`
Expected: PASS untuk semua package. Jika `GEMINI_API_KEY` / `ANTHROPIC_API_KEY` tidak diset, live test (`TestParseMessage_LiveAPI`, `TestGeminiProvider_GenerateLive`) akan skip — itu OK.

- [ ] **Step 3: (Opsional) Live test Gemini**

Set `GEMINI_API_KEY` di environment (jangan commit ke .env yang dipakai!). Run:
```bash
GEMINI_API_KEY=<your-key> go test ./internal/platform/llm/ -run TestGeminiProvider_GenerateLive -v -count=1
```
Expected: PASS dengan log response sapa Bahasa Indonesia dari Gemini.

- [ ] **Step 4: (Opsional) Manual smoke test end-to-end**

1. Set `LLM_PROVIDER=gemini` dan `GEMINI_API_KEY=<your-key>` di `backend/.env`
2. Jalankan: `go run ./cmd/server`
3. Dari terminal lain, kirim request manual ke endpoint `POST /api/message` (sesuai contract di `openspec/changes/add-message-ingestion/specs/message-ingestion/spec.md`), misalnya pakai `curl`:

   ```bash
   curl -X POST http://localhost:8080/api/message \
     -H "Content-Type: application/json" \
     -d '{
       "phone": "+6281234567890",
       "text": "makan siang sama temen 45rb",
       "received_at": "2026-05-15T12:00:00Z"
     }'
   ```
4. Expected: response 200 dengan `reply_type=confirm` (atau `error` kalau user belum terdaftar di DB — itu pun OK, asal bukan 5xx atau panic).

- [ ] **Step 5: Git log review**

Run: `git log --oneline -10`
Expected: melihat 7 commit baru (Task 1-7) sejak commit spec.

---

## Self-Review Notes (untuk implementor)

- **DRY**: Prompt builders (`prompt.go`) dan validator (`parser.go`) tetap di satu tempat — shared antar provider lewat composer.
- **YAGNI**: Tidak bikin abstraksi untuk multi-model routing per fungsi. Tidak bikin retry / circuit breaker custom — pakai default SDK.
- **TDD**: Setiap task baru dimulai dengan failing test, baru implementasi, baru pass. Constructor test (`RequiresAPIKey`) sudah cukup karena `Generate()` hard di-unit-test tanpa real API.
- **Frequent commits**: 7 commit untuk fitur ini, masing-masing self-contained dan reversible.
- **Tidak ada breaking change** untuk consumer interface `domain.MessageComposer` — semua usecase tidak perlu diubah.
