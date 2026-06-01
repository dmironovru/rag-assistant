package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
	"rag-assistant/internal/embedder"
)

type ServerConfig struct {
	DBConn     string `yaml:"db_connection"`
	OllamaURL  string `yaml:"ollama_url"`
	EmbedModel string `yaml:"embed_model"`
}

type SearchRequest struct {
	Query  string `json:"query"`
	Source string `json:"source,omitempty"` // фильтр по технологии
	Limit  int    `json:"limit"`
}

type SearchResult struct {
	Content    string  `json:"content"`
	Source     string  `json:"source"`
	Similarity float64 `json:"similarity"`
}

func main() {
	// 1. Загрузка конфига
	cfg, err := loadConfig("config/sources.yaml")
	if err != nil {
		log.Fatal("❌ Ошибка конфига:", err)
	}

	// 2. Подключение к БД
	db, err := pgxpool.New(context.Background(), cfg.DBConn)
	if err != nil {
		log.Fatal("❌ Ошибка подключения к БД:", err)
	}
	defer db.Close()
	log.Println("✅ Подключено к БД")

	// 3. Настройка роутов
	mux := http.NewServeMux()
	mux.HandleFunc("/api/search", corsMiddleware(searchHandler(db, cfg)))

	// Простой healthcheck
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf(" API Server запущен на http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
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

		log.Printf(" Поиск: %q (source=%s, limit=%d)", req.Query, req.Source, req.Limit)

		// 1. Генерируем эмбеддинг запроса
		queryEmb, err := embedder.EmbedText(req.Query, cfg.OllamaURL, cfg.EmbedModel)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"embedding failed: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// 2. Форматируем вектор для PostgreSQL
		embStr := formatVector(queryEmb)

		// 3. Выполняем векторный поиск
		// $2 = '' означает "не фильтровать по source"
		sql := `
			SELECT content, source, 1 - (embedding <=> $1::vector) as similarity
			FROM document_chunks
			WHERE ($2 = '' OR source = $2)
			ORDER BY embedding <=> $1::vector
			LIMIT $3
		`

		rows, err := db.Query(context.Background(), sql, embStr, req.Source, req.Limit)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"db query failed: %v"}`, err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var results []SearchResult
		for rows.Next() {
			var res SearchResult
			if err := rows.Scan(&res.Content, &res.Source, &res.Similarity); err != nil {
				log.Println("️ Ошибка сканирования строки:", err)
				continue
			}
			results = append(results, res)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// formatVector преобразует []float32 в строку "[0.1, 0.2, ...]" для pgvector
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

// corsMiddleware разрешает запросы с фронтенда (Next.js обычно на :3000)
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // В продакшене укажи точный домен
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
