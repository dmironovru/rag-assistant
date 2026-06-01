CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS document_chunks (
    id SERIAL PRIMARY KEY,
    source VARCHAR(100) NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT GENERATED ALWAYS AS (md5(content)) STORED,
    embedding vector(768),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(source, content_hash)
);

CREATE INDEX IF NOT EXISTS idx_embedding 
ON document_chunks USING hnsw (embedding vector_cosine_ops);