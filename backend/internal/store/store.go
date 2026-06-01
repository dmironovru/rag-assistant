package store

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SaveChunk сохраняет чанк с эмбеддингом в БД
func SaveChunk(db *pgxpool.Pool, sourceName string, chunkText string, embedding []float32) error {
	_, err := db.Exec(context.Background(),
		"INSERT INTO document_chunks (content, source, embedding) VALUES ($1, $2, $3)",
		chunkText, sourceName, vectorToString(embedding))
	return err
}

// GetDB возвращает подключение к БД
func GetDB(connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 10
	config.MinConns = 2

	db, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(context.Background()); err != nil {
		return nil, err
	}
	return db, nil
}

func vectorToString(vec []float32) string {
	str := "["
	for i, v := range vec {
		if i > 0 {
			str += ","
		}
		str += fmt.Sprintf("%f", v)
	}
	str += "]"
	return str
}
