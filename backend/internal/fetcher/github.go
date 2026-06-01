package fetcher

import (
	"fmt" // ← ДОБАВЛЕНО! Без этого fmt.Errorf не работает
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// CloneOrPull клонирует или обновляет репозиторий GitHub
func CloneOrPull(repoURL, branch, destDir string) error {
	// Конвертируем short format в полный URL
	if !strings.HasPrefix(repoURL, "http") {
		repoURL = "https://github.com/" + repoURL + ".git"
	}

	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		log.Printf("📥 Клонирование %s в %s...", repoURL, destDir)
		_, err := git.PlainClone(destDir, false, &git.CloneOptions{
			URL:           repoURL,
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			SingleBranch:  true,
			Depth:         1,
		})
		if err != nil {
			return fmt.Errorf("ошибка клонирования: %w", err)
		}
		log.Printf("✅ Клонирование завершено")
		return nil
	}

	log.Printf("🔄 Обновление репозитория в %s...", destDir)
	r, err := git.PlainOpen(destDir)
	if err != nil {
		return fmt.Errorf("ошибка открытия репозитория: %w", err)
	}
	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("ошибка получения worktree: %w", err)
	}
	err = w.Pull(&git.PullOptions{
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("ошибка pull: %w", err)
	}
	if err == git.NoErrAlreadyUpToDate {
		log.Printf("ℹ️ Репозиторий уже актуален")
	} else {
		log.Printf("✅ Обновление завершено")
	}
	return nil
}

// ReadFiles рекурсивно читает все .md и .mdx файлы из указанных путей
func ReadFiles(dir string, paths []string) ([]string, error) {
	var contents []string
	var totalFiles int

	for _, p := range paths {
		fullPath := filepath.Join(dir, p)

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			log.Printf("⚠️ Путь не найден: %s (пропускаем)", fullPath)
			continue
		}

		log.Printf("🔍 Сканирование: %s", fullPath)

		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("⚠️ Ошибка доступа к %s: %v", path, err)
				return nil
			}
			if info.IsDir() {
				return nil
			}
			name := strings.ToLower(info.Name())
			if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".mdx") {
				data, err := os.ReadFile(path)
				if err != nil {
					log.Printf("⚠️ Не удалось прочитать %s: %v", path, err)
					return nil
				}
				contents = append(contents, string(data))
				totalFiles++
				if totalFiles%20 == 0 {
					log.Printf("📄 Прочитано файлов: %d", totalFiles)
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("⚠️ Ошибка обхода пути %s: %v", fullPath, err)
		}
	}

	log.Printf("✅ Всего найдено файлов: %d", totalFiles)
	return contents, nil
}
