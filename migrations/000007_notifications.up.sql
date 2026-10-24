CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    actor_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type       text NOT NULL,
    post_id    uuid REFERENCES posts (id) ON DELETE CASCADE,
    comment_id uuid REFERENCES comments (id) ON DELETE CASCADE,
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT notifications_type_valid
        CHECK (type IN ('follow', 'comment', 'reaction')),
    CONSTRAINT notifications_no_self_notify CHECK (user_id <> actor_id),
    CONSTRAINT notifications_target_consistent CHECK (
        (type = 'follow' AND post_id IS NULL AND comment_id IS NULL)
        OR (type = 'comment' AND post_id IS NOT NULL AND comment_id IS NOT NULL)
        OR (type = 'reaction' AND post_id IS NOT NULL AND comment_id IS NULL)
    )
);

CREATE INDEX notifications_user_created_idx
    ON notifications (user_id, created_at DESC, id DESC);
CREATE INDEX notifications_user_unread_idx
    ON notifications (user_id, created_at DESC)
    WHERE read_at IS NULL;
