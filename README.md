Дмитрий, отличная заметка! Без `README.md` в корне репозиторий выглядит незавершённым. Давай создадим **полный, профессиональный README**, который будет служить документацией и визитной карточкой проекта для работодателей.

Создай файл в корне проекта:
```bash
nano ~/rag-portfolio/README.md
```

Вставь этот контент целиком:

```markdown
#  RAG Portfolio — AI-ассистент по документации

[![Go](https://img.shields.io/badge/Go-1.23-blue.svg)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-16-black.svg)](https://nextjs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-blue.svg)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **Полностью локальный AI-помощник**, который отвечает на вопросы по технической документации (React, Next.js, TypeScript).  
> Использует архитектуру **RAG (Retrieval-Augmented Generation)**: векторный поиск по документации + генерация ответов через локальную языковую модель. Никаких облачных API, все данные остаются на вашем компьютере.

 **Демо:** `git clone` → `./start.sh` → чат работает  
 **Репозиторий:** [github.com/dmironovru/rag-portfolio](https://github.com/dmironovru/rag-portfolio)

---

## 🎯 Возможности

- ✅ **Автоматическая индексация** документации из официальных репозиториев GitHub
- ✅ **Семантический поиск** через PostgreSQL + pgvector (векторные эмбеддинги)
- ✅ **Локальная генерация ответов** через Ollama (`mistral:7b`, `nomic-embed-text`)
- ✅ **Чат-интерфейс** на Next.js 16 с тёмной темой и подсветкой источников
- ✅ **Одна команда для запуска**: `./start.sh` поднимает всё окружение
- ✅ **Полностью приватно**: работает без интернета после первоначальной настройки

---

## 🏗️ Архитектура

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Frontend      │     │   Go Backend    │     │   Ollama        │
│   (Next.js 16)  │────▶│   (RAG API)     │────▶│   (LLM)         │
│   :3000         │────│   :8080         │────│   :11434        │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │   PostgreSQL + pgvector │
                    │   ~1800 чанков доков    │
                    └─────────────────────────
```

### Компоненты
| Компонент | Технология | Назначение |
|-----------|-----------|------------|
| **Frontend** | Next.js 16, React 19, Tailwind CSS | Чат-интерфейс, отправка вопросов, отображение ответов |
| **Backend** | Go 1.23, `chi`, `pgx`, `pgvector` | Векторный поиск, обработка запросов, управление чанками |
| **Embeddings** | Ollama + `nomic-embed-text` | Преобразование текста в векторы для поиска |
| **LLM** | Ollama + `mistral:7b` | Генерация связных ответов на основе найденных чанков |
| **Database** | PostgreSQL 16 + pgvector | Хранение чанков документации и векторных эмбеддингов |

---

## 🚀 Быстрый старт (рекомендуемый)

### Требования
| Компонент | Версия | Как проверить |
|-----------|--------|--------------|
| 🐹 Go | 1.22+ | `go version` |
|  Node.js | 20+ | `node --version` |
| 🐘 PostgreSQL | 16 + pgvector | `psql --version` |
| 🦙 Ollama | последняя | `ollama --version` |

>  **Не установлен Ollama?**  
> ```bash
> curl -fsSL https://ollama.com/install.sh | sh
> ```

### Запуск
```bash
# 1. Клонируй репозиторий
git clone https://github.com/dmironovru/rag-portfolio.git
cd rag-portfolio

# 2. Запусти всё одной командой
./start.sh

# 3. Открой в браузере
🌐 http://localhost:3000
```

### ⏱️ Что делает `./start.sh` при первом запуске?
1.  ✅ Проверяет наличие зависимостей (Go, Node, PostgreSQL, Ollama)
2.  ⬇️ Скачивает ИИ-модели (`~4.5 ГБ`), если их нет
3.  🗄️ Создаёт базу данных `rag_docs` и таблицы
4.  🚀 Запускает Ollama, Go-бэкенд и Next.js-фронтенд
5.  💡 Выводит ссылки и логи, корректно останавливается по `Ctrl+C`

> ⏳ **Первый запуск занимает 10-15 минут** (скачивание моделей). Повторные запуски — ~30 секунд.

### 🔍 Проверка работы
1.  Открой [http://localhost:3000](http://localhost:3000)
2.  В чате задай вопрос:  
    `что такое useEffect в React и как правильно указывать зависимости?`
3.  Подожди ~5-15 секунд
4.  ✅ Получи связный ответ с цитатами из документации и ссылками на источники

---

## 🛠️ Ручная настройка (если `./start.sh` не подходит)

### 1. Запусти Ollama и скачай модели
```bash
ollama serve
# В другом окне:
ollama pull nomic-embed-text
ollama pull mistral:7b
```

### 2. Создай базу данных
```bash
sudo -u postgres psql -c "CREATE DATABASE rag_docs;"
sudo -u postgres psql -d rag_docs -c "CREATE EXTENSION IF NOT EXISTS vector;"
# Инициализация таблиц:
sudo -u postgres psql -d rag_docs -f backend/deploy/init.sql
```

### 3. Запусти сервисы в отдельных терминалах
```bash
# Терминал 1: Бэкенд
cd backend
export DB_CONN="postgres://rag:ragpass@localhost:5432/rag_docs"
export OLLAMA_URL="http://localhost:11434"
go run cmd/server/main.go

# Терминал 2: Фронтенд
cd frontend
npm install
export RAG_API_URL="http://localhost:8080/api/search"
npm run dev
```

---

## ⚙️ Настройка и конфигурация

### Переменные окружения
Скопируй `.env.example` в `.env` и при необходимости измени значения:
```env
POSTGRES_USER=rag
POSTGRES_PASSWORD=ragpass
POSTGRES_DB=rag_docs
EMBED_MODEL=nomic-embed-text
LLM_MODEL=mistral:7b
```

### Доступные модели
| Модель | Размер | Назначение | Рекомендация |
|--------|--------|-----------|-------------|
| `nomic-embed-text` | ~270 MB | Эмбеддинги | ✅ По умолчанию |
| `mistral:7b` | ~4.1 GB | Генерация ответов | ✅ По умолчанию |
| `qwen2.5:7b` | ~4.2 GB | Лучшая поддержка русского | 🇷 Для вопросов на RU |
| `phi3:3.8b` | ~2.3 GB | Быстрая, лёгкая | ⚡ Для слабых машин |

**Смена модели:** измени `LLM_MODEL=...` в `.env` и перезапусти `./start.sh`.

---

## 📚 Заполнение базы знаний

По умолчанию база пустая. Чтобы добавить документацию:
```bash
cd backend
make ingest
```
Это загрузит и проиндексирует официальные доки React и Next.js из GitHub.  
Для добавления своих источников отредактируй `backend/config/sources.yaml`.

---

##  Частые вопросы

| Проблема | Решение |
|----------|---------|
| `Порт 8080/3000 занят` | Скрипт предложит освободить порт автоматически (нажми `1`) |
| `ENOENT: required-server-files.json` | Скрипт автоматически очищает `.next` кэш перед запуском |
| `Ответы на английском` | Смени модель на `qwen2.5:7b` в `.env` |
| `Генерация медленная` | Используй `phi3:3.8b` или включи GPU в настройках Ollama |
| `Docker не скачивает образы` | Используй нативный запуск `./start.sh` (он стабильнее в РФ) |

---

## 📦 Структура проекта
```
rag-portfolio/
├──  README.md                 # Документация
── 📄 start.sh                  # Запуск всего проекта одной командой
├──  deploy.sh                 # (Опционально) Docker-деплой
├──  docker-compose.yml        # Конфигурация контейнеров
├── 📁 backend/                  # Go-бэкенд (RAG API)
│   ├── 📁 cmd/                  # Точки входа (server, ingest)
│   ├── 📁 internal/             # Модули (chunker, embedder, fetcher, store)
│   └──  Makefile              # Удобные команды
── 📁 frontend/                 # Next.js-фронтенд (чат)
    ├── 📁 app/                  # Роуты и API
    └── 📄 package.json          # Зависимости
```

---

## 🤝 Вклад в проект
1.  Форкни репозиторий
2.  Создай ветку фичи: `git checkout -b feature/amazing`
3.  Закоммить изменения: `git commit -m 'Add amazing feature'`
4.  Запушь: `git push origin feature/amazing`
5.  Открой Pull Request

---

## 📄 Лицензия
MIT. См. файл [LICENSE](LICENSE).

---

## 👤 Автор
**Дмитрий Миронов** — Fullstack разработчик  
🌐 [dmitrymironov.ru](https://dmitrymironov.ru)  
🐙 [GitHub](https://github.com/dmironovru)  

> Этот проект создан для демонстрации навыков работы с современными технологиями:  
> **Go • Next.js • PostgreSQL • pgvector • Ollama • RAG-архитектура**
