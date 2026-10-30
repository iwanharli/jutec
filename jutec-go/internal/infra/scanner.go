package infra

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"net"
	"regexp"
	"strings"
)

type SiteInfra struct {
	IPs         []string
	NameServers []string
	MXRecords   []string
}

// TXTToken adalah token verifikasi yang diekstrak dari TXT record DNS.
type TXTToken struct {
	Service string // GoogleVerification | FacebookVerification | dll
	Token   string // nilai token-nya
	Raw     string // TXT record asli
}

type Scanner struct{}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) Scan(domain string) (*SiteInfra, error) {
	domain = cleanDomain(domain)

	infra := &SiteInfra{}

	ips, _ := net.LookupIP(domain)
	for _, ip := range ips {
		infra.IPs = append(infra.IPs, ip.String())
	}

	nss, _ := net.LookupNS(domain)
	for _, ns := range nss {
		infra.NameServers = append(infra.NameServers, ns.Host)
	}

	mxs, _ := net.LookupMX(domain)
	for _, mx := range mxs {
		infra.MXRecords = append(infra.MXRecords, mx.Host)
	}

	return infra, nil
}

// ScanTXT mengambil TXT records DNS dan mengekstrak token verifikasi.
// Token yang sama di banyak domain = satu operator.
func (s *Scanner) ScanTXT(rawURL string) []TXTToken {
	domain := cleanDomain(rawURL)

	records, err := net.LookupTXT(domain)
	if err != nil {
		return nil
	}

	var tokens []TXTToken
	seen := make(map[string]bool)

	for _, record := range records {
		extracted := parseTXTRecord(record)
		for _, t := range extracted {
			key := t.Service + ":" + t.Token
			if !seen[key] {
				seen[key] = true
				tokens = append(tokens, t)
			}
		}
	}

	return tokens
}

func (s *Scanner) GetSSLThumbprint(rawURL string) string {
	domain := cleanDomain(rawURL)

	conf := &tls.Config{InsecureSkipVerify: true}
	conn, err := tls.Dial("tcp", domain+":443", conf)
	if err != nil {
		return ""
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) > 0 {
		thumb := md5.Sum(certs[0].Raw)
		return hex.EncodeToString(thumb[:])
	}
	return ""
}

// txtRules mendefinisikan aturan ekstraksi per service.
var txtRules = []struct {
	Service string
	Pattern *regexp.Regexp
}{
	{
		Service: "GoogleVerification",
		Pattern: regexp.MustCompile(`(?i)google-site-verification=([A-Za-z0-9_\-]+)`),
	},
	{
		Service: "FacebookVerification",
		Pattern: regexp.MustCompile(`(?i)facebook-domain-verification=([A-Za-z0-9_\-]+)`),
	},
	{
		Service: "MicrosoftVerification",
		Pattern: regexp.MustCompile(`(?i)MS=([A-Za-z0-9]+)`),
	},
	{
		Service: "AppleVerification",
		Pattern: regexp.MustCompile(`(?i)apple-domain-verification=([A-Za-z0-9]+)`),
	},
	{
		Service: "YandexVerification",
		Pattern: regexp.MustCompile(`(?i)yandex-verification[=:\s]+([a-f0-9]+)`),
	},
	{
		Service: "SPFInclude",
		// Ekstrak domain dalam "include:domain.com" dari SPF record
		Pattern: regexp.MustCompile(`(?i)\binclude:([a-z0-9\.\-]+)`),
	},
}

func parseTXTRecord(record string) []TXTToken {
	var tokens []TXTToken

	for _, rule := range txtRules {
		for _, m := range rule.Pattern.FindAllStringSubmatch(record, -1) {
			if len(m) >= 2 && m[1] != "" {
				tokens = append(tokens, TXTToken{
					Service: rule.Service,
					Token:   strings.TrimSpace(m[1]),
					Raw:     record,
				})
			}
		}
	}

	return tokens
}

func cleanDomain(raw string) string {
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return strings.Split(raw, "/")[0]
}
