package crawler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"jutec-go/internal/browser"

	cdpnetwork "github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Mode Pantau: Jutec membuka Chrome yang TERLIHAT di layar analis, mengisi data dummy, lalu analis
// menyelesaikan captcha/OTP & menekan daftar sendiri sambil melihat hasilnya. Jutec merekam pertukaran
// jaringan sejak jendela dibuka; saat analis klik "Simpan hasil", keadaan halaman saat itu diambil.

type AttendedSession struct {
	PageURL string
	RegURL  string
	Filled  map[string]string
	Started time.Time

	allocCancel context.CancelFunc
	tabCancel   context.CancelFunc
	tab         context.Context
	mu          sync.Mutex
	ex          map[cdpnetwork.RequestID]*ProbeExchange
	order       []cdpnetwork.RequestID
}

// OpenAttended meluncurkan Chrome terlihat, membuka halaman daftar, dan mengisi data dummy (tanpa submit).
func (c *Crawler) OpenAttended(pageURL, actionHint string, prof ProbeProfile) (*AttendedSession, error) {
	rules, _ := dohResolverRules(context.Background(), pageURL)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false), // TERLIHAT — analis bisa solve captcha & submit
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("start-maximized", true),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"))
	if rules != "" {
		opts = append(opts, chromedp.Flag("host-resolver-rules", rules))
	}
	allocCtx, ac := browser.NewAllocator(context.Background(), opts...)
	tab, tc := chromedp.NewContext(allocCtx)

	s := &AttendedSession{PageURL: pageURL, RegURL: pageURL, Filled: map[string]string{}, Started: time.Now(),
		allocCancel: ac, tabCancel: tc, tab: tab, ex: map[cdpnetwork.RequestID]*ProbeExchange{}}

	// Rekam pertukaran jaringan sejak jendela dibuka
	chromedp.ListenTarget(tab, func(ev any) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch e := ev.(type) {
		case *cdpnetwork.EventRequestWillBeSent:
			t := string(e.Type)
			if e.Request.Method == "GET" && t != "Document" && t != "XHR" && t != "Fetch" {
				return
			}
			x := &ProbeExchange{URL: e.Request.URL, Method: e.Request.Method, Type: t}
			if e.Request.HasPostData {
				for _, en := range e.Request.PostDataEntries {
					if b, err := base64.StdEncoding.DecodeString(en.Bytes); err == nil {
						x.Request += string(b)
					}
				}
			}
			if _, ok := s.ex[e.RequestID]; !ok {
				s.order = append(s.order, e.RequestID)
			}
			s.ex[e.RequestID] = x
		case *cdpnetwork.EventResponseReceived:
			if x := s.ex[e.RequestID]; x != nil {
				x.Status = int(e.Response.Status)
				if x.Headers == nil {
					x.Headers = map[string]string{}
				}
				for _, h := range platformHeaders {
					for k, v := range e.Response.Headers {
						if strings.EqualFold(k, h) {
							x.Headers[h] = fmt.Sprint(v)
						}
					}
				}
			}
		}
	})

	var fill struct {
		Error   string            `json:"error"`
		Filled  map[string]string `json:"filled"`
		Missing []string          `json:"missing"`
	}
	profJSON := mustProfileJSON(prof)
	err := chromedp.Run(tab, cdpnetwork.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error { _, _, _, _, err := page.Navigate(pageURL).Do(ctx); return err }),
		chromedp.WaitReady("body", chromedp.ByQuery), chromedp.Sleep(4*time.Second),
		chromedp.Evaluate(fmt.Sprintf("%s(%s, %q)", probeFillJS, profJSON, actionHint), &fill),
	)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("gagal membuka halaman: %w", err)
	}
	s.Filled = fill.Filled
	return s, nil
}

// Capture mengambil keadaan halaman saat ini (setelah analis submit/login manual) sebagai hasil uji.
func (s *AttendedSession) Capture(c *Crawler) *ProbeResult {
	res := &ProbeResult{PageURL: s.PageURL, Status: "submitted", Outcome: "unknown",
		Filled: s.Filled, Missing: []string{}, Exchanges: []ProbeExchange{}, Artifacts: []TransactionArtifact{}, Platform: []string{}}
	if res.Filled == nil {
		res.Filled = map[string]string{}
	}
	// Ambil isi respons yang tercatat
	s.mu.Lock()
	ids := append([]cdpnetwork.RequestID{}, s.order...)
	s.mu.Unlock()
	for _, id := range ids {
		var body []byte
		_ = chromedp.Run(s.tab, chromedp.ActionFunc(func(ctx context.Context) error {
			b, err := cdpnetwork.GetResponseBody(id).Do(ctx)
			body = b
			return err
		}))
		s.mu.Lock()
		if x := s.ex[id]; x != nil {
			if len(body) > 8192 {
				body = body[:8192]
			}
			x.Body = string(body)
			res.Exchanges = append(res.Exchanges, *x)
		}
		s.mu.Unlock()
	}
	var msg string
	var st probeState
	_ = chromedp.Run(s.tab, chromedp.Location(&res.FinalURL), chromedp.Evaluate(probeMessageJS, &msg),
		chromedp.Evaluate(probeStateJS, &st),
		chromedp.OuterHTML("html", &res.HTMLAfter, chromedp.ByQuery), chromedp.CaptureScreenshot(&res.ShotAfter))
	res.Message = msg
	res.Artifacts = ExtractArtifacts(res.HTMLAfter)
	if res.Artifacts == nil {
		res.Artifacts = []TransactionArtifact{}
	}
	res.Outcome = probeOutcome(res, st, s.RegURL)
	res.Platform = probePlatform(res)
	return res
}

func (s *AttendedSession) Close() {
	if s.tabCancel != nil {
		s.tabCancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
	}
}

func mustProfileJSON(prof ProbeProfile) string { b, _ := json.Marshal(prof); return string(b) }

func (s *AttendedSession) Expired(d time.Duration) bool { return time.Since(s.Started) > d }
