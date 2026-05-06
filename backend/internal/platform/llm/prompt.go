package llm

import (
	"fmt"
	"strings"
	"time"

	"budgeting/internal/domain"
	"budgeting/pkg/currency"
)

const parseSystemPrompt = `Kamu adalah asisten keuangan personal. Tugasmu: pahami pesan bebas user dan kembalikan JSON terstruktur.

KATEGORI PENGELUARAN: Makan & Minum, Transport, Belanja, Tagihan, Hiburan, Kesehatan, Pendidikan, Lainnya
KATEGORI PEMASUKAN: Gaji, Freelance, Investasi, Hadiah, Lainnya

INTENT:
- expense     : pengeluaran ("makan siang 45rb", "grab 25k", "bensin 50ribu")
- income      : pemasukan ("gaji 5jt", "dapat freelance 500rb")
- balance     : cek saldo ("saldo berapa?", "sisa uang gue")
- report      : laporan ("laporan bulan ini", "pengeluaran minggu ini")
- delete_last : hapus transaksi terakhir ("hapus yang tadi", "batal", "salah input")
- set_goal    : target tabungan ("mau nabung buat laptop 10 juta", "nabung liburan 5jt")
- set_budget  : budget kategori ("budget makan 500rb", "limit transport 300ribu")
- check_goal  : cek progress ("nabung udah berapa?", "progress laptop gimana?")
- unknown     : tidak dikenali

ATURAN NOMINAL: rb/ribu/k = ×1.000 | jt/juta = ×1.000.000
Contoh: "50rb"=50000, "2jt"=2000000, "1.5jt"=1500000

OUTPUT (hanya JSON, tanpa teks lain):
{"intent":"expense","amount":45000,"category":"Makan & Minum","note":"makan siang sama temen","goal_name":"","confidence":0.95}

Turunkan confidence < 0.75 jika ragu.`

const reminderSystemPrompt = `Kamu adalah asisten keuangan personal yang ramah dan supportif.
Tugasmu: susun pesan WhatsApp yang singkat, natural, dan memotivasi berdasarkan data keuangan user.
Gunakan emoji secukupnya. Bahasa Indonesia informal tapi sopan. Maksimal 5-6 baris.`

func buildPostTransactionPrompt(rc domain.ReminderContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Transaksi baru dicatat:\n- Kategori: %s\n- Jumlah: Rp %s\n- Saldo sekarang: Rp %s\n",
		rc.LastCategory, currency.FormatIDR(rc.LastAmount), currency.FormatIDR(rc.Balance)))

	if sisa, ok := rc.BudgetRemaining[rc.LastCategory]; ok {
		sb.WriteString(fmt.Sprintf("- Sisa budget %s bulan ini: Rp %s\n", rc.LastCategory, currency.FormatIDR(sisa)))
	}

	if len(rc.SavingsGoals) > 0 {
		g := rc.SavingsGoals[0]
		pct := int64(0)
		if g.TargetAmount > 0 {
			pct = g.SavedAmount * 100 / g.TargetAmount
		}
		sb.WriteString(fmt.Sprintf("- Target tabungan '%s': Rp %s / Rp %s (%d%%)\n",
			g.Name, currency.FormatIDR(g.SavedAmount), currency.FormatIDR(g.TargetAmount), pct))
	}

	sb.WriteString("\nSusun pesan konfirmasi transaksi + konteks keuangan di atas. Singkat dan motivatif.")
	return sb.String()
}

func buildDailyReminderPrompt(rc domain.ReminderContext) string {
	var sb strings.Builder

	now := time.Now()
	sb.WriteString(fmt.Sprintf("Data keuangan untuk reminder harian (%s):\n", now.Format("Monday, 2 January 2006")))
	sb.WriteString(fmt.Sprintf("- Saldo: Rp %s\n", currency.FormatIDR(rc.Balance)))
	sb.WriteString(fmt.Sprintf("- Pengeluaran hari ini: Rp %s\n", currency.FormatIDR(rc.TodaySpend)))

	if len(rc.SpendByCategory) > 0 {
		sb.WriteString("- Breakdown hari ini:\n")
		for cat, amt := range rc.SpendByCategory {
			sb.WriteString(fmt.Sprintf("  • %s: Rp %s\n", cat, currency.FormatIDR(amt)))
		}
	}

	if len(rc.BudgetRemaining) > 0 {
		sb.WriteString("- Sisa budget bulan ini:\n")
		for cat, sisa := range rc.BudgetRemaining {
			sb.WriteString(fmt.Sprintf("  • %s: Rp %s\n", cat, currency.FormatIDR(sisa)))
		}
	}

	if len(rc.SavingsGoals) > 0 {
		sb.WriteString("- Target tabungan aktif:\n")
		for _, g := range rc.SavingsGoals {
			pct := int64(0)
			if g.TargetAmount > 0 {
				pct = g.SavedAmount * 100 / g.TargetAmount
			}
			sb.WriteString(fmt.Sprintf("  • %s: %d%% (Rp %s dari Rp %s)\n",
				g.Name, pct, currency.FormatIDR(g.SavedAmount), currency.FormatIDR(g.TargetAmount)))
		}
	}

	sb.WriteString("\nSusun pesan reminder harian yang ramah, informatif, dan motivatif.")
	return sb.String()
}
