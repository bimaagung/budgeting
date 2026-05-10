package usecase

import (
	"fmt"
	"strings"

	"budgeting/pkg/currency"
)

func replyClarifyLowConfidence() string {
	return "Maaf, saya kurang yakin maksudnya 🤔\n" +
		"Bisa tulis ulang? Contoh:\n" +
		"• *makan 50rb*\n" +
		"• *gaji 5jt*\n" +
		"• *nabung laptop 10jt*"
}

func replyClarifyUnknown() string {
	return "Saya belum bisa membantu dengan itu.\n" +
		"Coba: *makan 50rb*, *gaji 5jt*, atau *saldo berapa?*"
}

func replyClarifyMissingAmount(intent string) string {
	switch intent {
	case "set_goal":
		return "Mau nabung berapa (nominal)? Contoh: *nabung laptop 10 juta*"
	case "set_budget":
		return "Budgetnya berapa (nominal)? Contoh: *budget makan 500rb*"
	default:
		return "Mohon sertakan nominal."
	}
}

func replyReportNotAvailable() string {
	return "Fitur laporan belum tersedia, mohon tunggu update ✨"
}

func replyError() string {
	return "Maaf, ada gangguan sebentar. Coba lagi sebentar lagi 🙏"
}

func replyConfirmFallback(txType, category string, amount, balance int64) string {
	verb := "Tercatat"
	if txType == "income" {
		verb = "Pemasukan tercatat"
	}
	return fmt.Sprintf("✅ %s: %s Rp %s\n💰 Saldo: Rp %s",
		verb, category, currency.FormatIDR(amount), currency.FormatIDR(balance))
}

func replyBalance(balance int64) string {
	return fmt.Sprintf("💰 Saldo kamu saat ini: *Rp %s*", currency.FormatIDR(balance))
}

func replyDeleteLastSuccess(note string, amount int64) string {
	return fmt.Sprintf("🗑️ Transaksi dihapus: _%s_ Rp %s", note, currency.FormatIDR(amount))
}

func replyDeleteLastEmpty() string {
	return "Tidak ada transaksi yang bisa dihapus."
}

func replyCheckGoals(goals []MessageSavingsGoal) string {
	if len(goals) == 0 {
		return "Belum ada target tabungan aktif. Coba: *nabung buat laptop 10 juta*"
	}
	var sb strings.Builder
	sb.WriteString("🎯 *Progress Tabungan Kamu:*\n\n")
	for _, g := range goals {
		sb.WriteString(fmt.Sprintf("• *%s*\n  Target: Rp %s\n  Terkumpul: Rp %s (%d%%)\n\n",
			g.Name, currency.FormatIDR(g.Target), currency.FormatIDR(g.Saved), g.ProgressPct))
	}
	return sb.String()
}
