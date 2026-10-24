CREATE TABLE follows (
    follower_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    following_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (follower_id, following_id),
    CONSTRAINT follows_no_self_follow CHECK (follower_id <> following_id)
);

-- The primary key covers "who do I follow"; this index covers "who follows me".
CREATE INDEX follows_following_idx ON follows (following_id, created_at DESC, follower_id);
