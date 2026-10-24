CREATE TABLE posts (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content      text NOT NULL,
    image_url    text,
    status       text NOT NULL DEFAULT 'published',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,

    CONSTRAINT posts_content_length
        CHECK (char_length(content) BETWEEN 1 AND 500),
    CONSTRAINT posts_image_url_format
        CHECK (image_url IS NULL OR image_url ~* '^https?://'),
    CONSTRAINT posts_status_valid
        CHECK (status IN ('draft', 'published')),
    CONSTRAINT posts_published_at_consistent
        CHECK (
            (status = 'published' AND published_at IS NOT NULL)
            OR (status = 'draft' AND published_at IS NULL)
        )
);

-- Keyset pagination orders by (created_at, id); the matching composite
-- indexes let PostgreSQL walk the index instead of sorting.
CREATE INDEX posts_author_created_idx
    ON posts (author_id, created_at DESC, id DESC);
CREATE INDEX posts_published_created_idx
    ON posts (created_at DESC, id DESC)
    WHERE status = 'published';
