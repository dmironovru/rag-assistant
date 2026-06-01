#!/bin/bash
set -e

echo "🔍 Проверка целостности RAG Portfolio..."

# 1. Проверка структуры
echo "1️⃣ Проверяем наличие файлов..."
FILES=("backend/cmd/server/main.go" "backend/cmd/ingest/main.go" "backend/go.mod" "backend/deploy/Dockerfile.api" "frontend/package.json" "docker-compose.yml" "deploy.sh")
for f in "${FILES[@]}"; do
  if [ ! -f "$f" ]; then
    echo "❌ Отсутствует: $f"
    exit 1
  else
    echo "✅ Найдено: $f"
  fi
done

# 2. КРИТИЧЕСКАЯ ПРОВЕРКА: Совпадает ли имя модуля в Go?
# Если мы переименовали папку в backend, а в go.mod осталось rag-assistant - импорты сломаются
echo ""
echo "2️⃣ Проверяем Go Module Name..."
MODULE_NAME=$(grep "^module" backend/go.mod | awk '{print $2}')
echo "   Модуль: $MODULE_NAME"

# Проверим, есть ли импорты, ссылающиеся на старое имя, если модуль уже новый
# Или наоборот
echo "   ⚠️ Внимание: Если ты переименовал папку в 'backend', убедись, что в go.mod module 'backend' (или оставь 'rag-assistant')!"

# 3. Проверка Next.js API Route
echo ""
echo "3️⃣ Проверяем API роут в Next.js..."
if [ -f "frontend/app/api/ask/route.ts" ]; then
    echo "✅ Роут ask/route.ts найден"
else
    echo "❌ Роут frontend/app/api/ask/route.ts НЕ НАЙДЕН (чат не будет работать!)"
fi

# 4. Проверка Dockerfile в Frontend
echo ""
echo "4️⃣ Проверяем Dockerfile в Frontend..."
if [ -f "frontend/Dockerfile" ]; then
    echo "✅ Dockerfile найден"
else
    echo "❌ Dockerfile в папке frontend отсутствует!"
fi

echo ""
echo " Предварительная проверка завершена."