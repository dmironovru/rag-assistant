package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"unicode"
	"unicode/utf8"

	"rag-assistant/internal/embedder"

	"github.com/axgle/mahonia"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DBConn     string `yaml:"db_connection"`
	OllamaURL  string `yaml:"ollama_url"`
	EmbedModel string `yaml:"embed_model"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run cmd/ingest/main.go <file_path>")
	}

	filePath := os.Args[1]

	cfg, err := loadConfig("config/sources.yaml")
	if err != nil {
		log.Fatal("❌ Ошибка конфига:", err)
	}

	db, err := pgxpool.New(context.Background(), cfg.DBConn)
	if err != nil {
		log.Fatal("❌ Ошибка подключения к БД:", err)
	}
	defer db.Close()
	log.Println("✅ Подключено к БД")

	log.Printf("📄 Индексация файла: %s", filePath)

	text, err := extractText(filePath)
	if err != nil {
		log.Fatal("❌ Ошибка извлечения текста:", err)
	}

	text = convertToUTF8(text)
	text = cleanText(text)

	if len(text) < 10 {
		log.Fatal("❌ Текст слишком короткий или пустой")
	}

	log.Printf("📝 Размер текста: %d символов", len(text))

	// Разбивка по символам, а не по словам!
	chunks := splitIntoChunksByChars(text, 300, 50)
	log.Printf("📦 Создано %d чанков", len(chunks))

	saved := 0
	for i, chunk := range chunks {
		if len(chunk) < 10 {
			continue
		}

		emb, err := embedder.EmbedText(chunk, cfg.OllamaURL, cfg.EmbedModel)
		if err != nil {
			log.Printf("⚠️ Ошибка эмбеддинга чанка %d: %v", i, err)
			continue
		}

		_, err = db.Exec(context.Background(),
			"INSERT INTO document_chunks (content, embedding, source) VALUES ($1, $2, $3)",
			chunk, formatVector(emb), filePath,
		)
		if err != nil {
			log.Printf("⚠️ Ошибка сохранения чанка %d: %v", i, err)
			continue
		}
		saved++

		if saved%50 == 0 {
			log.Printf("✅ Сохранено %d/%d чанков", saved, len(chunks))
		}
	}

	log.Printf("🎉 Индексация завершена! Сохранено %d/%d чанков", saved, len(chunks))
}

func extractText(filePath string) (string, error) {
	ext := strings.ToLower(filePath[strings.LastIndex(filePath, "."):])

	switch ext {
	case ".pdf":
		return extractTextFromPDF(filePath)
	case ".doc", ".docx":
		return extractTextFromDOC(filePath)
	default:
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

func extractTextFromPDF(filePath string) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("pdftotext not installed: %w", err)
	}

	// Вариант 1: с сохранением layout (-layout)
	cmd := exec.Command("pdftotext", "-layout", "-enc", "UTF-8", filePath, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		text := stdout.String()
		if len(strings.TrimSpace(text)) > 100 {
			return text, nil
		}
	}

	// Вариант 2: без layout (-raw)
	cmd = exec.Command("pdftotext", "-raw", "-enc", "UTF-8", filePath, "-")
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		text := stdout.String()
		if len(strings.TrimSpace(text)) > 100 {
			return text, nil
		}
	}

	// Вариант 3: с фиксом ширины (-fixed 10)
	cmd = exec.Command("pdftotext", "-fixed", "10", "-enc", "UTF-8", filePath, "-")
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		text := stdout.String()
		if len(strings.TrimSpace(text)) > 100 {
			return text, nil
		}
	}

	return "", fmt.Errorf("extracted text is too short (probably scanned PDF or encrypted)")
}

func extractTextFromDOC(filePath string) (string, error) {
	if _, err := exec.LookPath("catdoc"); err == nil {
		cmd := exec.Command("catdoc", filePath)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err == nil {
			text := stdout.String()
			if len(strings.TrimSpace(text)) > 10 {
				return text, nil
			}
		}
	}

	if _, err := exec.LookPath("antiword"); err == nil {
		cmd := exec.Command("antiword", filePath)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err == nil {
			text := stdout.String()
			if len(strings.TrimSpace(text)) > 10 {
				return text, nil
			}
		}
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ============================================================
// РАЗБИВКА НА ЧАНКИ ПО СИМВОЛАМ
// ============================================================
func splitIntoChunksByChars(text string, size int, overlap int) []string {
	if len(text) == 0 {
		return []string{}
	}

	var chunks []string
	textRunes := []rune(text)
	length := len(textRunes)

	step := size - overlap
	if step <= 0 {
		step = size
	}

	for i := 0; i < length; i += step {
		end := i + size
		if end > length {
			end = length
		}
		chunk := string(textRunes[i:end])
		if len(strings.TrimSpace(chunk)) > 0 {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

func convertToUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	decoder := mahonia.NewDecoder("windows-1251")
	if decoder != nil {
		converted := decoder.ConvertString(s)
		if utf8.ValidString(converted) {
			return converted
		}
	}
	return strings.ToValidUTF8(s, " ")
}

func cleanText(text string) string {
	var result strings.Builder
	for _, r := range text {
		if r == utf8.RuneError {
			result.WriteRune(' ')
		} else if unicode.IsPrint(r) || unicode.IsSpace(r) {
			result.WriteRune(r)
		} else {
			result.WriteRune(' ')
		}
	}
	return result.String()
}

func formatVector(v []float32) string {
	var b strings.Builder
	b.WriteString("[")
	for i, f := range v {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%.6f", f))
	}
	b.WriteString("]")
	return b.String()
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
