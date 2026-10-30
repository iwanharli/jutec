package store

import (
	"context"
	"os"
	"strings"
)

// APIKeyDef: kunci API sumber data yang bisa diatur dari halaman Pengaturan.
// Nilai di DB (settings "secret:<ENV>") menimpa .env; diterapkan ke environment saat backend mulai.
type APIKeyDef struct {
	Env      string `json:"env"`
	Label    string `json:"label"`
	Group    string `json:"group"` // pencarian | reputasi | identitas | blockchain | telegram
	Desc     string `json:"desc"`
	Signup   string `json:"signup,omitempty"`
	Required bool   `json:"required,omitempty"`
	FreeTier bool   `json:"free_tier,omitempty"` // tetap jalan tanpa kunci (kuota terbatas)
	Secret   bool   `json:"secret"`              // disamarkan di UI
}

var APIKeyDefs = []APIKeyDef{
	{Env: "SERPER_API_KEY", Label: "Serper (Google)", Group: "pencarian", Desc: "Pencarian Google untuk Deep Sweep & dork. Wajib untuk penemuan situs.", Signup: "https://serper.dev", Required: true, Secret: true},
	{Env: "SERPAPI_API_KEY", Label: "SerpAPI", Group: "pencarian", Desc: "Cadangan pencarian Google saat kuota Serper habis.", Signup: "https://serpapi.com", Secret: true},
	{Env: "BRAVE_API_KEY", Label: "Brave Search", Group: "pencarian", Desc: "Mesin pencari kedua (indeks independen dari Google) — menemukan situs yang tak muncul di Google. Kuota gratis ±2.000 kueri/bulan.", Signup: "https://api-dashboard.search.brave.com/register", FreeTier: false, Secret: true},
	{Env: "URLSCAN_API_KEY", Label: "URLScan", Group: "reputasi", Desc: "Riwayat scan & screenshot situs dari urlscan.io.", Signup: "https://urlscan.io/user/signup", Secret: true},
	{Env: "VIRUSTOTAL_API_KEY", Label: "VirusTotal", Group: "reputasi", Desc: "Reputasi domain dari puluhan vendor keamanan.", Signup: "https://www.virustotal.com/gui/join-us", Secret: true},
	{Env: "HACKERTARGET_API_KEY", Label: "HackerTarget", Group: "reputasi", Desc: "Cari situs lain yang memakai ID tracking yang sama.", Signup: "https://hackertarget.com/scan-membership/", FreeTier: true, Secret: true},
	{Env: "LEAKOSINT_TOKEN", Label: "LeakOSINT", Group: "identitas", Desc: "Pemilik nomor HP & rekening dari data bocoran.", Secret: true},
	{Env: "TRONGRID_API_KEY", Label: "TronGrid", Group: "blockchain", Desc: "Jejak transaksi dompet USDT (TRON).", Signup: "https://www.trongrid.io", FreeTier: true, Secret: true},
	{Env: "ETHERSCAN_API_KEY", Label: "Etherscan", Group: "blockchain", Desc: "Jejak transaksi dompet USDT di Ethereum (ERC20). Paket gratis tidak mencakup BSC.", Signup: "https://etherscan.io/myapikey", Secret: true},
	{Env: "TELEGRAM_API_ID", Label: "Telegram API ID", Group: "telegram", Desc: "Membaca channel Telegram situs judol. Dari my.telegram.org.", Signup: "https://my.telegram.org/apps"},
	{Env: "TELEGRAM_API_HASH", Label: "Telegram API Hash", Group: "telegram", Desc: "Pasangan API ID di atas.", Signup: "https://my.telegram.org/apps", Secret: true},
	{Env: "TELEGRAM_PHONE", Label: "Nomor akun Telegram", Group: "telegram", Desc: "Nomor akun yang dipakai membaca channel."},
}

const secretPrefix = "secret:"

func APIKeyDefByEnv(env string) *APIKeyDef {
	for i := range APIKeyDefs {
		if APIKeyDefs[i].Env == env {
			return &APIKeyDefs[i]
		}
	}
	return nil
}

// ApplyAPIKeys memuat kunci dari DB ke environment (menimpa .env). Dipanggil sebelum config dibaca.
func (db *PostgresDB) ApplyAPIKeys(ctx context.Context) int {
	rows, err := db.Pool.Query(ctx, `SELECT key, value FROM settings WHERE key LIKE 'secret:%' AND value <> ''`)
	if err != nil {
		return 0
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil && APIKeyDefByEnv(strings.TrimPrefix(k, secretPrefix)) != nil {
			os.Setenv(strings.TrimPrefix(k, secretPrefix), v)
			n++
		}
	}
	return n
}

type APIKeyStatus struct {
	APIKeyDef
	Set     bool   `json:"set"`
	Source  string `json:"source"` // db | env | "" (belum diisi)
	Preview string `json:"preview"`
	Pending bool   `json:"pending"` // nilai DB belum diterapkan ke proses yang berjalan
}

func mask(v string, secret bool) string {
	if !secret {
		return v
	}
	if len(v) <= 8 {
		return strings.Repeat("•", len(v))
	}
	return v[:4] + strings.Repeat("•", 6) + v[len(v)-4:]
}

func (db *PostgresDB) APIKeyStatuses(ctx context.Context) []APIKeyStatus {
	stored := map[string]string{}
	if rows, err := db.Pool.Query(ctx, `SELECT key, value FROM settings WHERE key LIKE 'secret:%'`); err == nil {
		for rows.Next() {
			var k, v string
			if rows.Scan(&k, &v) == nil {
				stored[strings.TrimPrefix(k, secretPrefix)] = v
			}
		}
		rows.Close()
	}
	out := make([]APIKeyStatus, 0, len(APIKeyDefs))
	for _, d := range APIKeyDefs {
		st := APIKeyStatus{APIKeyDef: d}
		running := os.Getenv(d.Env)
		if v := stored[d.Env]; v != "" {
			st.Set, st.Source, st.Preview, st.Pending = true, "db", mask(v, d.Secret), v != running
		} else if running != "" {
			st.Set, st.Source, st.Preview = true, "env", mask(running, d.Secret)
		}
		out = append(out, st)
	}
	return out
}

// SetAPIKey menyimpan kunci; nilai kosong menghapus override DB (kembali ke .env).
func (db *PostgresDB) SetAPIKey(ctx context.Context, env, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		_, err := db.Pool.Exec(ctx, `DELETE FROM settings WHERE key = $1`, secretPrefix+env)
		return err
	}
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO settings (key, value, description, updated_at) VALUES ($1, $2, 'Kunci API (diatur dari Pengaturan)', NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, secretPrefix+env, value)
	return err
}
