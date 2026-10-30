// Package trustpositif mencocokkan domain dengan daftar blokir TrustPositif (Komdigi).
// Server resmi sering tak terjangkau dari luar jaringan ISP, jadi dipakai mirror harian
// github.com/alsyundawy/TrustPositif (berkas domains_isp.7z = daftar yang diterapkan ISP).
package trustpositif

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
)

const DefaultURL = "https://raw.githubusercontent.com/alsyundawy/TrustPositif/main/domains_isp.7z"

// Match mengunduh daftar lalu mengembalikan host mana dari `hosts` yang terdaftar (streaming —
// daftar berisi jutaan domain dan tidak disimpan). total = jumlah baris daftar.
func Match(ctx context.Context, url string, hosts map[string]bool) (listed map[string]bool, total int, err error) {
	tmp, err := os.CreateTemp("", "trustpositif-*.7z")
	if err != nil {
		return nil, 0, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	res, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, 0, fmt.Errorf("unduh daftar: HTTP %d", res.StatusCode)
	}
	size, err := io.Copy(tmp, res.Body)
	if err != nil {
		return nil, 0, err
	}

	zr, err := sevenzip.NewReader(tmp, size)
	if err != nil {
		return nil, 0, fmt.Errorf("buka arsip: %w", err)
	}
	listed = map[string]bool{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, 0, err
		}
		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			d := strings.ToLower(strings.TrimSpace(sc.Text()))
			if d == "" || d[0] == '#' {
				continue
			}
			total++
			d = strings.TrimPrefix(strings.TrimPrefix(d, "*."), "www.")
			if hosts[d] {
				listed[d] = true
			}
		}
		rc.Close()
		if err := sc.Err(); err != nil {
			return nil, 0, err
		}
	}
	return listed, total, nil
}
