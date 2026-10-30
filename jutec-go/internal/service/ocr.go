package service

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type OCRService struct {
	Client *http.Client
}

func NewOCRService() *OCRService {
	return &OCRService{
		Client: &http.Client{
			Timeout: 15 * time.Second, // Timeout untuk download gambar
		},
	}
}

// ExtractText mendownload gambar dari URL dan menjalankan Tesseract OCR.
// Mengembalikan teks hasil ekstrak atau error.
func (s *OCRService) ExtractText(imageURL string) (string, error) {
	// 1. Buat temporary file
	tmpFile, err := os.CreateTemp("", "jutec_ocr_*.img")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Bersihkan file setelah selesai

	// 2. Download gambar
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		tmpFile.Close()
		return "", err
	}
	// Tambahkan header standar untuk menghindari blokir 403
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := s.Client.Do(req)
	if err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return "", fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close() // Tutup file agar bisa dibaca oleh tesseract
	if err != nil {
		return "", fmt.Errorf("failed to save image data: %w", err)
	}

	// 3. Jalankan Tesseract CLI
	// Command: tesseract <file> stdout -l ind+eng
	cmd := exec.Command("tesseract", tmpPath, "stdout", "-l", "ind+eng")
	out, err := cmd.Output()
	if err != nil {
		// Jika Tesseract gagal (misalnya gambar rusak/bukan gambar)
		return "", fmt.Errorf("tesseract error: %w", err)
	}

	// 4. Bersihkan dan kembalikan teks
	text := strings.TrimSpace(string(out))
	return text, nil
}
