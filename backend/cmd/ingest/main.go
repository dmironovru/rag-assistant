package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"rag-assistant/internal/chunker"
	"rag-assistant/internal/embedder"
	"rag-assistant/internal/fetcher"
	"rag-assistant/internal/store"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DBConn     string   `yaml:"db_connection"`
	OllamaURL  string   `yaml:"ollama_url"`
	EmbedModel string   `yaml:"embed_model"`
	Sources    []Source `yaml:"sources"`
}

type Source struct {
	Name         string   `yaml:"name"`
	Type         string   `yaml:"type"`
	Repo         string   `yaml:"repo"`
	Branch       string   `yaml:"branch"`
	Paths        []string `yaml:"paths"`
	URL          string   `yaml:"url"`
	Format       string   `yaml:"format"`
	Language     string   `yaml:"language"`
	ChunkSize    int      `yaml:"chunk_size"`
	ChunkOverlap int      `yaml:"chunk_overlap"`
}

func main() {
	configPath := flag.String("config", "config/sources.yaml", "Путь к конфигу")
	filter := flag.String("filter", "", "Обработать только этот источник")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal("Ошибка загрузки конфига:", err)
	}

	db, err := store.GetDB(cfg.DBConn)
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}
	defer db.Close()
	log.Println("✅ Подключено к БД")

	for _, src := range cfg.Sources {
		if *filter != "" && src.Name != *filter {
			continue
		}

		log.Printf("📥 Обработка: %s", src.Name)

		var documents []string

		if src.Type == "github" {
			// Конвертируем short format в полный URL
			repoURL := src.Repo
			if !strings.HasPrefix(repoURL, "https://") {
				repoURL = "https://github.com/" + repoURL + ".git"
			}

			cacheDir := filepath.Join("cache", src.Name)
			if err := fetcher.CloneOrPull(repoURL, src.Branch, cacheDir); err != nil {
				log.Printf("⚠️ Ошибка клонирования %s: %v", src.Name, err)
				continue
			}

			documents, err = fetcher.ReadFiles(cacheDir, src.Paths)
			if err != nil {
				log.Printf("⚠️ Ошибка чтения файлов %s: %v", src.Name, err)
				continue
			}
			log.Printf("📄 %s: %d файлов", src.Name, len(documents))
		}

		totalChunks := 0
		for _, doc := range documents {
			// Очищаем frontmatter из markdown
			cleanDoc := cleanMarkdown(doc)
			chunks := chunker.Split(cleanDoc, src.ChunkSize, src.ChunkOverlap)
			totalChunks += len(chunks)

			for i, chunk := range chunks {
				embedding, err := embedder.EmbedText(chunk, cfg.OllamaURL, cfg.EmbedModel)
				if err != nil {
					log.Printf("⚠️ Ошибка эмбеддинга чанка %d: %v", i, err)
					continue
				}

				if err := store.SaveChunk(db, src.Name, chunk, embedding); err != nil {
					log.Printf("⚠️ Ошибка сохранения чанка %d: %v", i, err)
					continue
				}
			}
		}

		log.Printf("✅ %s: готово (%d чанков)", src.Name, totalChunks)
	}

	log.Println("🎉 Все источники обработаны!")
}

func cleanMarkdown(content string) string {
	// Убираем frontmatter (--- ... ---)
	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[2])
	}
	return content
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
