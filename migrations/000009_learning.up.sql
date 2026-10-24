-- Learning platform persistence. Content itself stays in Markdown; these tables
-- hold only learner activity keyed by stable slugs.

CREATE TABLE learn_profiles (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    guest_hash   bytea UNIQUE,
    display_name text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT learn_profiles_one_owner
        CHECK ((user_id IS NOT NULL) <> (guest_hash IS NOT NULL))
);

COMMENT ON TABLE learn_profiles IS
    'One row per learner. Authenticated profiles reference users; guest profiles are bound to a hashed browser cookie.';

-- Browser sessions for the server-rendered learning interface. Only a hash of
-- the cookie value is stored, so a database leak does not hand out sessions.
CREATE TABLE learn_sessions (
    token_hash   bytea PRIMARY KEY,
    profile_id   uuid NOT NULL REFERENCES learn_profiles (id) ON DELETE CASCADE,
    user_id      uuid REFERENCES users (id) ON DELETE CASCADE,
    user_agent   text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,

    CONSTRAINT learn_sessions_owner
        CHECK ((user_id IS NULL) OR (user_id IS NOT NULL))
);

CREATE INDEX learn_sessions_expires_idx ON learn_sessions (expires_at);
CREATE INDEX learn_sessions_profile_idx ON learn_sessions (profile_id);

CREATE TABLE learn_lesson_progress (
    profile_id      uuid NOT NULL REFERENCES learn_profiles (id) ON DELETE CASCADE,
    lesson_slug     text NOT NULL,
    status          text NOT NULL DEFAULT 'in_progress',
    reading_percent smallint NOT NULL DEFAULT 0,
    first_opened_at timestamptz NOT NULL DEFAULT now(),
    last_opened_at  timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz,

    PRIMARY KEY (profile_id, lesson_slug),

    CONSTRAINT learn_progress_status_valid
        CHECK (status IN ('in_progress', 'completed')),
    CONSTRAINT learn_progress_percent_range
        CHECK (reading_percent BETWEEN 0 AND 100),
    CONSTRAINT learn_progress_completed_consistent
        CHECK (
            (status = 'completed' AND completed_at IS NOT NULL)
            OR (status = 'in_progress' AND completed_at IS NULL)
        )
);

CREATE INDEX learn_progress_recent_idx
    ON learn_lesson_progress (profile_id, last_opened_at DESC);

CREATE TABLE learn_bookmarks (
    profile_id  uuid NOT NULL REFERENCES learn_profiles (id) ON DELETE CASCADE,
    lesson_slug text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (profile_id, lesson_slug)
);

CREATE TABLE learn_notes (
    profile_id  uuid NOT NULL REFERENCES learn_profiles (id) ON DELETE CASCADE,
    lesson_slug text NOT NULL,
    body        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (profile_id, lesson_slug),
    CONSTRAINT learn_notes_body_length CHECK (char_length(body) <= 20000)
);

CREATE TABLE learn_quiz_attempts (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id  uuid NOT NULL REFERENCES learn_profiles (id) ON DELETE CASCADE,
    quiz_slug   text NOT NULL,
    lesson_slug text NOT NULL,
    score       integer NOT NULL,
    total       integer NOT NULL,
    answers     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT learn_quiz_score_range CHECK (score >= 0 AND score <= total),
    CONSTRAINT learn_quiz_total_positive CHECK (total > 0)
);

CREATE INDEX learn_quiz_attempts_profile_idx
    ON learn_quiz_attempts (profile_id, created_at DESC);

CREATE TABLE learn_exercise_attempts (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id    uuid NOT NULL REFERENCES learn_profiles (id) ON DELETE CASCADE,
    exercise_slug text NOT NULL,
    status        text NOT NULL DEFAULT 'submitted',
    score         integer NOT NULL DEFAULT 0,
    total         integer NOT NULL DEFAULT 0,
    response      text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT learn_exercise_status_valid
        CHECK (status IN ('started', 'submitted', 'completed')),
    CONSTRAINT learn_exercise_response_length
        CHECK (char_length(response) <= 20000),
    CONSTRAINT learn_exercise_score_range
        CHECK (score >= 0 AND (total = 0 OR score <= total))
);

CREATE INDEX learn_exercise_attempts_profile_idx
    ON learn_exercise_attempts (profile_id, exercise_slug, created_at DESC);

CREATE TABLE learn_preferences (
    profile_id     uuid PRIMARY KEY REFERENCES learn_profiles (id) ON DELETE CASCADE,
    theme          text NOT NULL DEFAULT 'auto',
    font_scale     text NOT NULL DEFAULT 'default',
    reduced_motion boolean NOT NULL DEFAULT false,
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT learn_preferences_theme_valid
        CHECK (theme IN ('auto', 'light', 'dark')),
    CONSTRAINT learn_preferences_font_valid
        CHECK (font_scale IN ('default', 'large'))
);
