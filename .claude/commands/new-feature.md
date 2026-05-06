
# Skill: new-feature

Rencanakan dan implementasi fitur baru secara end-to-end lintas semua layer.

## Tugas

Fitur baru: **$ARGUMENTS**

## Analisis yang Harus Dilakukan Sebelum Implementasi

1. **Apakah fitur ini dipicu dari WhatsApp, Flutter, atau keduanya?**
2. **Apakah butuh perubahan skema database?** Jika ya, buat migration file di `backend/migrations/`.
3. **Apakah butuh n8n workflow baru atau modifikasi existing?**

## Urutan Implementasi (ikuti Clean Architecture)

```
1. Migration SQL            → backend/migrations/
2. Domain entity + interface → internal/domain/
3. Platform implementation  → internal/platform/postgres/  (implements domain interface)
4. Usecase                  → internal/usecase/             (depends on domain interface)
5. Handler + route          → internal/handler/             (depends on usecase)
6. LLM prompt update        → internal/platform/llm/        (jika ada WA command baru)
7. n8n workflow note
8. Flutter screen / widget  → mobile/lib/features/          (jika ada UI baru)
```

## Checklist Sebelum Selesai

- [ ] Unit test untuk usecase layer (mock domain interface)
- [ ] Response WA sudah ramah dan singkat
- [ ] Flutter screen menggunakan format Rupiah dan tanggal `id_ID`
- [ ] Tidak ada HTTP call langsung di dalam Flutter widget
- [ ] Handler tidak import `platform` langsung — hanya lewat usecase
- [ ] `domain` tidak mengimport package manapun dari `internal`
