# 🚀 Деплой RAG-ассистента

## Локально (WSL)

```bash
# 1. Запусти сервисы по отдельности (см. выше)
# 2. Открой http://localhost:3000

## На VPS (одной командой)

```bash
# 1. Склонируй репо
git clone <твой-репозиторий> my-rag
cd my-rag

# 2. Запусти всё через Docker
docker compose -f docker-compose.full.yml up -d

# 3. Скачай модели (только первый раз!)
docker compose exec ollama ollama pull nomic-embed-text
docker compose exec ollama ollama pull mistral:7b

# 4. Готово! Открой http://<твой-IP>:3000