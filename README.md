# RAG Assistant — AI-ассистент по документам

[![Go](https://img.shields.io/badge/Go-1.23-blue.svg)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-16-black.svg)](https://nextjs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-blue.svg)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **Полностью локальный AI-помощник**, который отвечает на вопросы по вашим документам (TXT, PDF, DOC).  
> Использует архитектуру **RAG (Retrieval-Augmented Generation)**: векторный поиск + генерация ответов через локальную LLM.  
> **Никаких облачных API** — все данные остаются на вашем компьютере.

🎯 **Демо:** `git clone` → `./start.sh` → чат работает  
📦 **Репозиторий:** [github.com/dmironovru/rag-assistant](https://github.com/dmironovru/rag-assistant)

---

## ✨ Возможности

- 📄 **Загрузка документов** — TXT, PDF, DOC/DOCX через интерфейс
- 🔍 **Векторный поиск** — PostgreSQL + pgvector для семантического поиска
- 🧠 **Локальный AI** — Ollama с моделями `nomic-embed-text` и `mistral:7b`
- 💬 **Чат-интерфейс** — Next.js 16 с тёмной темой и источниками ответов
- 🚀 **Одна команда** — `./start.sh` поднимает всё окружение
- 🔒 **Полностью приватно** — работает без интернета после настройки

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
                    │   Векторные эмбеддинги  │
                    └─────────────────────────┘
```

| Компонент | Технология | Назначение |
|-----------|-----------|------------|
| **Frontend** | Next.js 16, React 19, Tailwind | Чат-интерфейс, загрузка файлов |
| **Backend** | Go 1.23, `pgx`, `pgvector` | Векторный поиск, API, индексация |
| **Embeddings** | Ollama + `nomic-embed-text` | Преобразование текста в векторы |
| **LLM** | Ollama + `mistral:7b` | Генерация ответов на основе контекста |
| **Database** | PostgreSQL 16 + pgvector | Хранение чанков и эмбеддингов |

---

## 🚀 Быстрый старт

### Требования

| Компонент | Версия | Проверка |
|-----------|--------|----------|
| 🐹 Go | 1.22+ | `go version` |
| 📦 Node.js | 20+ | `node --version` |
| 🐘 PostgreSQL | 16 + pgvector | `psql --version` |
| 🦙 Ollama | последняя | `ollama --version` |

> **Нет Ollama?** Установи: `curl -fsSL https://ollama.com/install.sh | sh`

### Запуск

```bash
# 1. Клонируй репозиторий
git clone https://github.com/dmironovru/rag-assistant.git
cd rag-assistant

# 2. Запусти всё одной командой
./start.sh

# 3. Открой в браузере
🌐 http://localhost:3000
```

### ⏱️ Что делает `./start.sh`

1. ✅ Проверяет зависимости (Go, Node, PostgreSQL, Ollama)
2. ⬇️ Скачивает AI-модели (~4.5 ГБ), если их нет
3. 🗄️ Создаёт базу данных `rag_docs` и таблицы
4. 🚀 Запускает Ollama, Go-бэкенд и Next.js-фронтенд
5. 💡 Показывает логи и ссылки

> ⏳ **Первый запуск:** 10–15 минут (скачивание моделей)  
> ⚡ **Повторные запуски:** ~30 секунд

---

## 📚 Демо-данные

В репозитории есть демо-файл для тестирования:

- **Гарри Поттер и Тайная комната** (русский перевод)  
  📂 `backend/storage/uploads/garri-potter-i-tajnaya-komnata.txt`

### Как использовать

1. Запусти систему: `./start.sh`
2. Открой `http://localhost:3000`
3. Нажми **"📄 Загрузить файл"**
4. Выбери файл из папки `backend/storage/uploads/`
5. Задавай вопросы, например:
   - _"Кто такой Гарри Поттер?"_
   - _"Что такое Тайная комната?"_
   - _"Кто такой Добби?"_
   - _"Кто такой Хагрид?"_

---

## 🧪 Проверка работы

```bash
# Задать вопрос через curl
curl -X POST http://localhost:8080/api/ask \
  -H "Content-Type: application/json" \
  -d '{"question":"Кто такой Добби?"}' | jq '.'
```

**Ожидаемый ответ:**
```json
{
  "answer": "Добби - это домовик, слуга семьи Малфой.",
  "sources": [
    "./storage/uploads/garri-potter-i-tajnaya-komnata.txt (similarity: 0.85)",
    ...
  ]
}
```

---

## 🛠️ Ручная настройка

Если `./start.sh` не подходит:

```bash
# 1. Запусти Ollama и скачай модели
ollama serve
ollama pull nomic-embed-text
ollama pull mistral:7b

# 2. Создай базу данных
sudo -u postgres psql -c "CREATE DATABASE rag_docs;"
sudo -u postgres psql -d rag_docs -c "CREATE EXTENSION IF NOT EXISTS vector;"

# 3. Запусти бэкенд
cd backend
export DB_CONN="postgres://rag:ragpass@localhost:5432/rag_docs"
go run cmd/server/main.go

# 4. Запусти фронтенд (в другом терминале)
cd frontend
npm install
npm run dev
```

---

## 🔧 Конфигурация

### Переменные окружения

Создай `.env` в корне проекта:

```env
DB_CONN=postgres://rag:ragpass@localhost:5432/rag_docs
OLLAMA_URL=http://localhost:11434
EMBED_MODEL=nomic-embed-text
GENERATE_MODEL=mistral:7b
```

### Доступные модели

| Модель | Размер | Назначение | Рекомендация |
|--------|--------|-----------|-------------|
| `nomic-embed-text` | 274 MB | Эмбеддинги | ✅ По умолчанию |
| `mistral:7b` | 4.1 GB | Генерация | ✅ По умолчанию |
| `qwen2.5:7b` | 4.2 GB | Русский язык | 🇷 Для вопросов на RU |
| `llama3.2:3b` | 2.0 GB | Быстрая | ⚡ Для слабых машин |

---

## ❓ Частые вопросы

| Проблема | Решение |
|----------|---------|
| **Порт 8080/3000 занят** | Скрипт предложит освободить порт (нажми `1`) |
| **Ответы на английском** | Смени модель на `qwen2.5:7b` в `.env` |
| **Медленная генерация** | Используй `llama3.2:3b` или включи GPU |
| **PDF не читается** | Установи `poppler-utils`: `sudo apt install poppler-utils` |
| **DOC не читается** | Установи `catdoc`: `sudo apt install catdoc` |

---

## 📁 Структура проекта

```
rag-assistant/
├── 📄 start.sh                 # Запуск одной командой
├── 📄 stop.sh                  # Остановка всех сервисов
├── 📁 backend/
│   ├── 📁 cmd/
│   │   ├── server/             # API сервер
│   │   └── ingest/             # Индексация файлов
│   ├── 📁 internal/
│   │   ├── chunker/            # Разбивка на чанки
│   │   ├── embedder/           # Эмбеддинги через Ollama
│   │   └── store/              # Работа с БД
│   ├── 📁 storage/uploads/     # Загруженные файлы
│   └── 📄 config/sources.yaml  # Конфигурация
├── 📁 frontend/
│   ├── 📁 app/
│   │   ├── api/ask/            # Эндпоинт вопросов
│   │   ├── api/upload/         # Эндпоинт загрузки
│   │   └── page.tsx            # Чат-интерфейс
│   └── 📄 package.json
└── 📄 README.md
```

---

## 🔒 Что НЕ попадает в репозиторий

- `~/.ollama/` — модели AI (~5 ГБ)
- `backend/cache/` — кэш документов
- `node_modules/`, `.next/` — зависимости Next.js
- `logs/` — логи
- `.env` — секреты (пароли, ключи)

Эти папки создаются автоматически при первом запуске `./start.sh`.

---

## 👤 Автор

**Дмитрий Миронов** — Fullstack разработчик  
🌐 [dmitrymironov.ru](https://dmitrymironov.ru)  
🐙 [GitHub](https://github.com/dmironovru)

> Этот проект создан для демонстрации навыков работы с:  
> **Go • Next.js • PostgreSQL • pgvector • Ollama • RAG-архитектура**
```
