CREATE TABLE comments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id    uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    author_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT comments_content_length
        CHECK (char_length(content) BETWEEN 1 AND 1000)
);

CREATE INDEX comments_post_created_idx ON comments (post_id, created_at ASC, id ASC);
CREATE INDEX comments_author_idx ON comments (author_id, created_at DESC);
