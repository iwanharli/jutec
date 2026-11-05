package handlers

import (
	"net/http"
	"strings"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

// RegisterFormsHandler: daftar form pendaftaran situs judol valid untuk analisis pola.
type RegisterFormsHandler struct {
	PG *store.PostgresDB
}

// GET /data/register-forms?scope=valid|verified&q=&field=a,b&bank=&captcha=yes|no&otp=&phone=&need_bank=&external=&live=1
func (h *RegisterFormsHandler) List(c *gin.Context) {
	f := store.RegisterFormFilter{
		Kind: c.Query("kind"), Pattern: c.Query("pattern"), Scope: c.Query("scope"), Query: c.Query("q"), Bank: c.Query("bank"),
		Captcha: c.Query("captcha"), OTP: c.Query("otp"), Phone: c.Query("phone"),
		NeedBank: c.Query("need_bank"), External: c.Query("external"), Live: c.Query("live") == "1",
	}
	if v := strings.TrimSpace(c.Query("field")); v != "" {
		f.Fields = strings.Split(v, ",")
	}
	res, err := h.PG.RegisterForms(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}
