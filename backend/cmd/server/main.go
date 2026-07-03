package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

type ServerConfig struct {
	DBConn        string `yaml:"db_connection"`
	OllamaURL     string `yaml:"ollama_url"`
	EmbedModel    string `yaml:"embed_model"`
	GenerateModel string `yaml:"generate_model"`
}

type AskRequest struct {
	Question string `json:"question"`
}

type AskResponse struct {
	Answer  string   `json:"answer"`
	Sources []string `json:"sources,omitempty"`
}

type SearchRequest struct {
	Query  string `json:"query"`
	Source string `json:"source,omitempty"`
	Limit  int    `json:"limit"`
}

type SearchResult struct {
	Content    string  `json:"content"`
	Source     string  `json:"source"`
	Similarity float64 `json:"similarity"`
}

func main() {
	cfg, err := loadConfig("config/sources.yaml")
	if err != nil {
		log.Fatal("❌ Ошибка конфига:", err)
	}

	if cfg.GenerateModel == "" {
		cfg.GenerateModel = "mistral:7b"
	}

	db, err := pgxpool.New(context.Background(), cfg.DBConn)
	if err != nil {
		log.Fatal("❌ Ошибка подключения к БД:", err)
	}
	defer db.Close()
	log.Println("✅ Подключено к БД")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/upload", corsMiddleware(uploadHandler(db, cfg)))
	mux.HandleFunc("/api/search", corsMiddleware(searchHandler(db, cfg)))
	mux.HandleFunc("/api/ask", corsMiddleware(askHandler(db, cfg)))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 API Server запущен на http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func uploadHandler(db *pgxpool.Pool, cfg *ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to read file: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		uploadDir := "./storage/uploads"
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			http.Error(w, "Failed to create upload directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		filePath := uploadDir + "/" + header.Filename
		dst, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "Failed to create file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(dst, file); err != nil {
			dst.Close()
			http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		dst.Close()

		chunkCount, err := indexFile(filePath, db, cfg)
		if err != nil {
			log.Printf("⚠️ Ошибка индексации %s: %v", header.Filename, err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":  "File uploaded, but indexing failed",
				"filename": header.Filename,
				"chunks":   0,
				"error":    err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  "File uploaded and indexed successfully",
			"filename": header.Filename,
			"path":     filePath,
			"chunks":   chunkCount,
		})
	}
}

func indexFile(filePath string, db *pgxpool.Pool, cfg *ServerConfig) (int, error) {
	log.Printf("📄 Начинаю индексацию: %s", filePath)

	var text string
	var err error

	ext := strings.ToLower(filePath[strings.LastIndex(filePath, "."):])

	switch ext {
	case ".pdf":
		text, err = extractTextFromPDF(filePath)
		if err != nil {
			return 0, fmt.Errorf("PDF extraction failed: %w", err)
		}
	case ".doc", ".docx":
		text, err = extractTextFromDOC(filePath)
		if err != nil {
			return 0, fmt.Errorf("DOC extraction failed: %w", err)
		}
	case ".txt", ".md", ".go", ".py", ".js", ".ts", ".json", ".yaml", ".yml", ".csv", ".xml":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return 0, fmt.Errorf("failed to read text file: %w", err)
		}
		text = string(data)
	default:
		data, e := os.ReadFile(filePath)
		if e != nil {
			return 0, fmt.Errorf("unsupported file type: %s", ext)
		}
		text = string(data)
	}

	text = convertToUTF8(text)
	text = cleanText(text)

	if len(text) < 10 {
		return 0, fmt.Errorf("extracted text is too short or empty")
	}

	log.Printf("📝 Размер текста: %d символов", len(text))

	// Разбивка по символам, а не по словам!
	chunks := splitIntoChunksByChars(text, 300, 50)
	log.Printf("📦 Создано %d чанков для %s", len(chunks), filePath)

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
	return saved, nil
}

// ============================================================
// РАЗБИВКА НА ЧАНКИ ПО СИМВОЛАМ (НОВАЯ ФУНКЦИЯ)
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

// ============================================================
// ИЗВЛЕЧЕНИЕ ТЕКСТА ИЗ PDF (УЛУЧШЕННАЯ ВЕРСИЯ)
// ============================================================
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

// ============================================================
// ИЗВЛЕЧЕНИЕ ТЕКСТА ИЗ DOC/DOCX
// ============================================================
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

func askHandler(db *pgxpool.Pool, cfg *ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req AskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		if req.Question == "" {
			http.Error(w, `{"error":"question is empty"}`, http.StatusBadRequest)
			return
		}

		log.Printf("❓ Вопрос: %q", req.Question)

		queryEmb, err := embedder.EmbedText(req.Question, cfg.OllamaURL, cfg.EmbedModel)
		if err != nil {
			log.Printf("❌ Ошибка эмбеддинга: %v", err)
			writeJSON(w, AskResponse{Answer: "Не удалось обработать вопрос."})
			return
		}

		sql := `
            SELECT content, source, 1 - (embedding <=> $1::vector) as similarity
            FROM document_chunks
            ORDER BY embedding <=> $1::vector
            LIMIT 10
        `

		rows, err := db.Query(context.Background(), sql, formatVector(queryEmb))
		if err != nil {
			log.Printf("❌ Ошибка запроса к БД: %v", err)
			writeJSON(w, AskResponse{Answer: "Ошибка базы данных."})
			return
		}
		defer rows.Close()

		var contexts []string
		var sources []string
		for rows.Next() {
			var content, source string
			var similarity float64
			if err := rows.Scan(&content, &source, &similarity); err != nil {
				continue
			}
			contexts = append(contexts, content)
			sources = append(sources, fmt.Sprintf("%s (similarity: %.2f)", source, similarity))
		}

		if len(contexts) == 0 {
			writeJSON(w, AskResponse{
				Answer:  "В базе знаний нет информации по вашему вопросу. Загрузите документы.",
				Sources: []string{},
			})
			return
		}

		context := strings.Join(contexts, "\n---\n")

		// ПРОСТОЙ ПРОМПТ (как в самом начале, когда работало 50/50)
		prompt := fmt.Sprintf(`Отвечай на вопрос, используя только информацию из контекста.
Если в контексте нет ответа — скажи: "В контексте нет информации".

Контекст:
%s

Вопрос: %s

Ответ:`, context, req.Question)

		answer, err := generateWithOllama(cfg.OllamaURL, cfg.GenerateModel, prompt)
		if err != nil {
			log.Printf("❌ Ошибка генерации: %v", err)
			writeJSON(w, AskResponse{
				Answer:  "Не удалось сгенерировать ответ. Попробуйте позже.",
				Sources: sources,
			})
			return
		}

		writeJSON(w, AskResponse{
			Answer:  answer,
			Sources: sources,
		})
	}
}

func generateWithOllama(ollamaURL, model, prompt string) (string, error) {
	payload := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.1, // Очень низкая температура для точных ответов
			"top_p":       0.8,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return strings.TrimSpace(result.Response), nil
}

func searchHandler(db *pgxpool.Pool, cfg *ServerConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		if req.Limit == 0 {
			req.Limit = 5
		}

		queryEmb, err := embedder.EmbedText(req.Query, cfg.OllamaURL, cfg.EmbedModel)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"embedding failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		sql := `
            SELECT content, source, 1 - (embedding <=> $1::vector) as similarity
            FROM document_chunks
            WHERE ($2 = '' OR source = $2)
            ORDER BY embedding <=> $1::vector
            LIMIT $3
        `

		rows, err := db.Query(context.Background(), sql, formatVector(queryEmb), req.Source, req.Limit)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"db query failed: %v"}`, err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []SearchResult
		for rows.Next() {
			var res SearchResult
			if err := rows.Scan(&res.Content, &res.Source, &res.Similarity); err != nil {
				continue
			}
			results = append(results, res)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func loadConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
