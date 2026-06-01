#!/bin/bash
set -e

echo "🚀 RAG Portfolio: Автоматический деплой"
echo "========================================"

# 1. Проверка Docker
if ! command -v docker &> /dev/null; then
    echo "⚠️ Docker не найден. Устанавливаем..."
    sudo apt update
    sudo apt install -y docker.io docker-compose-plugin
    sudo systemctl enable --now docker
    echo "✅ Docker установлен"
fi

# 2. Настройка .env
if [ ! -f .env ]; then
    echo "📝 Создаём .env из шаблона..."
    cp .env.example .env
    echo "💡 Совет: для продакшена смени пароль в .env"
fi

# 3. Запуск инфраструктуры
echo " Запускаем контейнеры..."
docker compose up -d

# 4. Ожидание готовности Ollama
echo "⏳ Ожидание запуска Ollama..."
sleep 10
until docker compose exec -T ollama ollama list &> /dev/null; do
    echo "   Ждём инициализацию..."
    sleep 3
done

# 5. Скачивание моделей
echo " Скачиваем модели (может занять время)..."
docker compose exec -T ollama ollama pull nomic-embed-text
docker compose exec -T ollama ollama pull mistral:7b

# 6. Финальный статус
echo ""
echo "✅ Деплой завершён!"
echo "🌐 Фронтенд: http://localhost:3000"
echo " RAG API:  http://localhost:8080/health"
echo ""
echo "📌 Управление:"
echo "   docker compose ps          # Статус"
echo "   docker compose logs -f     # Логи"
echo "   docker compose down        # Остановка"
echo ""
echo " Заполнить базу знаний:"
echo "   docker compose run --rm backend make ingest"
