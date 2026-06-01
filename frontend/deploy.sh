#!/bin/bash
# deploy.sh — Деплой RAG-ассистента на чистый Ubuntu VPS

set -e
echo "🚀 Деплой RAG Assistant..."

# 1. Системные зависимости
sudo apt update && sudo apt install -y docker.io docker-compose git

# 2. Клонируем репо (если нужно)
if [ ! -d "my-rag" ]; then
    git clone <твой-репозиторий> my-rag
fi
cd my-rag

# 3. Переменные окружения
cat > .env <<EOF
# Модели (скачаются при первом запуске)
EMBED_MODEL=nomic-embed-text
LLM_MODEL=mistral:7b

# БД
DB_USER=rag
DB_PASS=ragpass
DB_NAME=rag_docs
EOF

# 4. Запускаем инфраструктуру
docker compose -f docker-compose.prod.yml up -d postgres ollama

# 5. Ждём готовности
echo "⏳ Ожидание сервисов..."
sleep 15

# 6. Скачиваем модели (только первый раз!)
echo "🧠 Скачивание моделей..."
docker compose exec ollama ollama pull $EMBED_MODEL
docker compose exec ollama ollama pull $LLM_MODEL

# 7. Запускаем API и фронтенд
docker compose -f docker-compose.prod.yml up -d rag-api frontend

# 8. Проверка
echo "✅ Проверка..."
sleep 5
curl -f http://localhost:3000 >/dev/null && echo "🌐 Фронтенд: ОК" || echo "⚠️ Фронтенд: ещё грузится"
curl -f http://localhost:8080/health >/dev/null && echo "🔌 RAG API: ОК" || echo "⚠️ RAG API: ещё грузится"

echo ""
echo "🎉 Готово!"
echo "🌐 Открой: http://<твой-IP>:3000"
echo "📚 База знаний: $(docker compose exec postgres psql -U rag -d rag_docs -t -c 'SELECT COUNT(*) FROM document_chunks;' | tr -d ' ') чанков"
echo "🧠 Модели: $EMBED_MODEL + $LLM_MODEL"
echo ""
echo "💡 Для обновления: git pull && docker compose -f docker-compose.prod.yml up -d --build"