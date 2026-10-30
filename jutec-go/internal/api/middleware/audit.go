package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

// Akses baca yang tetap dicatat karena menyangkut data pribadi atau bukti.
var auditReadPaths = []string{"/leak", "/data/kominfo", "/data/dukcapil", "/evidence", "/audit", "/ips"}
var reIdentities = regexp.MustCompile(`^/sites/[^/]+/(identities|evidence)`)

// Audit mencatat setiap perubahan data & akses ke data sensitif ke log audit berantai hash.
// actorOf diberikan dari luar agar paket ini tak bergantung pada handlers.
func Audit(pg *store.PostgresDB, actorOf func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if pg == nil || c.Request.Method == http.MethodOptions {
			return
		}
		p := c.Request.URL.Path
		sensitive := reIdentities.MatchString(p)
		for _, pre := range auditReadPaths {
			if strings.HasPrefix(p, pre) {
				sensitive = true
			}
		}
		if c.Request.Method == http.MethodGet && !sensitive {
			return
		}
		route := c.FullPath()
		if route == "" {
			route = p
		}
		target, _ := c.Get("audit_target")
		t, _ := target.(string)
		if t == "" {
			if id := c.Param("id"); id != "" {
				kind := "site"
				switch {
				case strings.HasPrefix(route, "/evidence"):
					kind = "evidence"
				case strings.HasPrefix(route, "/blacklist"):
					kind = "blacklist"
				}
				t = fmt.Sprintf("%s:%s", kind, id)
			}
		}
		detail := map[string]any{}
		if q := c.Request.URL.RawQuery; q != "" {
			detail["query"] = q
		}
		if n := len(c.Errors); n > 0 {
			detail["error"] = c.Errors.Last().Error()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := pg.Audit(ctx, actorOf(c), c.ClientIP(), c.Request.Method+" "+route, t, c.Writer.Status(), detail); err != nil {
			log.Printf("⚠️  [Audit] gagal mencatat %s %s: %v", c.Request.Method, p, err)
		}
	}
}
