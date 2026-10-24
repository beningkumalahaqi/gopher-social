-- Full-text search without an external engine. 'simple' avoids English
-- stemming so usernames and short posts are matched literally.
CREATE INDEX users_search_idx ON users
    USING GIN (to_tsvector('simple', username || ' ' || display_name));

CREATE INDEX posts_search_idx ON posts
    USING GIN (to_tsvector('english', content));
