CREATE TABLE reactions (
    post_id    uuid NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    -- The primary key is the business rule: one active reaction per user/post.
    PRIMARY KEY (post_id, user_id),
    CONSTRAINT reactions_type_valid CHECK (type IN ('like', 'love', 'laugh'))
);

CREATE INDEX reactions_post_type_idx ON reactions (post_id, type);
