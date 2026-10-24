CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username      text NOT NULL,
    display_name  text NOT NULL DEFAULT '',
    email         text NOT NULL,
    password_hash text NOT NULL,
    bio           text NOT NULL DEFAULT '',
    avatar_url    text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT users_username_format
        CHECK (username = lower(username) AND username ~ '^[a-z0-9_]{3,30}$'),
    CONSTRAINT users_email_format
        CHECK (email = lower(email) AND position('@' IN email) > 1),
    CONSTRAINT users_display_name_length
        CHECK (char_length(display_name) <= 60),
    CONSTRAINT users_bio_length
        CHECK (char_length(bio) <= 160),
    CONSTRAINT users_avatar_url_format
        CHECK (avatar_url IS NULL OR avatar_url ~* '^https?://')
);

-- A plain UNIQUE constraint would be enough for equality lookups; the named
-- indexes make the intent obvious and are what the query planner uses.
CREATE UNIQUE INDEX users_username_key ON users (username);
CREATE UNIQUE INDEX users_email_key ON users (email);

-- Prefix search on usernames ("al" -> "alice") uses this index because the
-- database collation is C-like for the pattern (username is lowercase ASCII).
CREATE INDEX users_username_pattern_idx ON users (username text_pattern_ops);
