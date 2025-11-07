CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename VARCHAR(255) NOT NULL,
    original_path VARCHAR(512) NOT NULL,
    content_type VARCHAR(100),
    size_bytes BIGINT NOT NULL,
    width INTEGER,
    height INTEGER,
    uploaded_by VARCHAR(100),
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'uploaded',
    metadata JSONB
);

CREATE TABLE processed_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_image_id UUID REFERENCES images(id) ON DELETE CASCADE,
    operation_type VARCHAR(100) NOT NULL,
    processed_path VARCHAR(512) NOT NULL,
    parameters JSONB,
    processing_time_ms INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_images_uploaded_at ON images(uploaded_at);
CREATE INDEX idx_images_status ON images(status);
CREATE INDEX idx_processed_original_id ON processed_images(original_image_id);