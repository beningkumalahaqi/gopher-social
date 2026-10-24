-- Development seed data. Every seeded account uses the password:
--   Password123!
-- This script is destructive: it clears all application tables first.

TRUNCATE users, refresh_tokens, posts, follows, comments, reactions, notifications CASCADE;

INSERT INTO users (id, username, display_name, email, password_hash, bio, avatar_url) VALUES
('11111111-1111-4111-8111-111111111111', 'alice', 'Alice Johnson', 'alice@example.com',
 '$argon2id$v=19$m=65536,t=3,p=2$N+p4uloEeJwdLkCn6BLNDw$k1ecnn6T3xx1yXdNKIAIpjKYqDEAYRUt/iHxEygmu94',
 'Backend engineer learning Go in public. Postgres apologist.', 'https://api.dicebear.com/9.x/initials/svg?seed=Alice'),
('22222222-2222-4222-8222-222222222222', 'bob', 'Bob Martinez', 'bob@example.com',
 '$argon2id$v=19$m=65536,t=3,p=2$N+p4uloEeJwdLkCn6BLNDw$k1ecnn6T3xx1yXdNKIAIpjKYqDEAYRUt/iHxEygmu94',
 'Database nerd. EXPLAIN ANALYZE is my love language.', NULL),
('33333333-3333-4333-8333-333333333333', 'carol', 'Carol Nguyen', 'carol@example.com',
 '$argon2id$v=19$m=65536,t=3,p=2$N+p4uloEeJwdLkCn6BLNDw$k1ecnn6T3xx1yXdNKIAIpjKYqDEAYRUt/iHxEygmu94',
 'Concurrency, coffee, and small interfaces.', NULL),
('44444444-4444-4444-8444-444444444444', 'dave', 'Dave Okafor', 'dave@example.com',
 '$argon2id$v=19$m=65536,t=3,p=2$N+p4uloEeJwdLkCn6BLNDw$k1ecnn6T3xx1yXdNKIAIpjKYqDEAYRUt/iHxEygmu94',
 'Runs before sunrise. Ships before lunch.', NULL),
('55555555-5555-4555-8555-555555555555', 'erin', 'Erin Park', 'erin@example.com',
 '$argon2id$v=19$m=65536,t=3,p=2$N+p4uloEeJwdLkCn6BLNDw$k1ecnn6T3xx1yXdNKIAIpjKYqDEAYRUt/iHxEygmu94',
 'Security engineer. Passwords are not encryption.', NULL),
('66666666-6666-4666-8666-666666666666', 'frank', 'Frank Silva', 'frank@example.com',
 '$argon2id$v=19$m=65536,t=3,p=2$N+p4uloEeJwdLkCn6BLNDw$k1ecnn6T3xx1yXdNKIAIpjKYqDEAYRUt/iHxEygmu94',
 'New here. Curious about everything.', NULL);

INSERT INTO posts (id, author_id, content, image_url, status, created_at, updated_at, published_at) VALUES
('a1111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-111111111111',
 'Just shipped my first Go API. net/http really does get you further than you expect.',
 NULL, 'published', now() - interval '6 days', now() - interval '6 days', now() - interval '6 days'),
('a2222222-2222-4222-8222-222222222222', '11111111-1111-4111-8111-111111111111',
 'Draft: benchmarking pgx connection pool sizes under load.',
 NULL, 'draft', now() - interval '3 days', now() - interval '3 days', NULL),
('a3333333-3333-4333-8333-333333333333', '11111111-1111-4111-8111-111111111111',
 'Re-reading Effective Go. The section on embedding finally clicked this time.',
 NULL, 'published', now() - interval '2 days', now() - interval '2 days', now() - interval '2 days'),
('b1111111-1111-4111-8111-111111111111', '22222222-2222-4222-8222-222222222222',
 'EXPLAIN ANALYZE before you add an index. Half my "slow query" tickets were missing WHERE clauses, not indexes.',
 NULL, 'published', now() - interval '5 days', now() - interval '5 days', now() - interval '5 days'),
('b2222222-2222-4222-8222-222222222222', '22222222-2222-4222-8222-222222222222',
 'Hot take: boring technology is a feature. PostgreSQL, a queue table, and a cron job can carry a lot of product.',
 NULL, 'published', now() - interval '1 day', now() - interval '1 day', now() - interval '1 day'),
('b3333333-3333-4333-8333-333333333333', '22222222-2222-4222-8222-222222222222',
 'Draft: notes on transaction isolation levels, with pictures.',
 NULL, 'draft', now() - interval '4 hours', now() - interval '4 hours', NULL),
('c1111111-1111-4111-8111-111111111111', '33333333-3333-4333-8333-333333333333',
 'Built a tiny worker pool today. Buffered channel, fixed goroutines, context-aware shutdown. Less code than I expected.',
 NULL, 'published', now() - interval '4 days', now() - interval '4 days', now() - interval '4 days'),
('c2222222-2222-4222-8222-222222222222', '33333333-3333-4333-8333-333333333333',
 'Go interfaces are not inheritance. If you find yourself building a hierarchy, stop and compose instead.',
 NULL, 'published', now() - interval '3 days', now() - interval '3 days', now() - interval '3 days'),
('d1111111-1111-4111-8111-111111111111', '44444444-4444-4444-8444-444444444444',
 'Sunrise run done. 8km along the river. The hardest part is leaving the house.',
 'https://images.unsplash.com/photo-1470252649378-9c29740c9fa8?w=800',
 'published', now() - interval '8 hours', now() - interval '8 hours', now() - interval '8 hours'),
('e1111111-1111-4111-8111-111111111111', '55555555-5555-4555-8555-555555555555',
 'Argon2id vs bcrypt: both fine, different trade-offs. Use a maintained library and tune the cost.',
 NULL, 'published', now() - interval '12 hours', now() - interval '12 hours', now() - interval '12 hours'),
('f1111111-1111-4111-8111-111111111111', '66666666-6666-4666-8666-666666666666',
 'Hello GopherSocial. First post, mostly to test the feed.',
 NULL, 'published', now() - interval '30 minutes', now() - interval '30 minutes', now() - interval '30 minutes');

INSERT INTO follows (follower_id, following_id, created_at) VALUES
('11111111-1111-4111-8111-111111111111', '22222222-2222-4222-8222-222222222222', now() - interval '10 days'),
('11111111-1111-4111-8111-111111111111', '33333333-3333-4333-8333-333333333333', now() - interval '9 days'),
('11111111-1111-4111-8111-111111111111', '44444444-4444-4444-8444-444444444444', now() - interval '7 days'),
('22222222-2222-4222-8222-222222222222', '11111111-1111-4111-8111-111111111111', now() - interval '10 days'),
('22222222-2222-4222-8222-222222222222', '33333333-3333-4333-8333-333333333333', now() - interval '8 days'),
('33333333-3333-4333-8333-333333333333', '11111111-1111-4111-8111-111111111111', now() - interval '9 days'),
('44444444-4444-4444-8444-444444444444', '11111111-1111-4111-8111-111111111111', now() - interval '7 days'),
('44444444-4444-4444-8444-444444444444', '22222222-2222-4222-8222-222222222222', now() - interval '6 days'),
('55555555-5555-4555-8555-555555555555', '11111111-1111-4111-8111-111111111111', now() - interval '5 days'),
('55555555-5555-4555-8555-555555555555', '22222222-2222-4222-8222-222222222222', now() - interval '5 days'),
('55555555-5555-4555-8555-555555555555', '33333333-3333-4333-8333-333333333333', now() - interval '5 days'),
('66666666-6666-4666-8666-666666666666', '11111111-1111-4111-8111-111111111111', now() - interval '20 minutes');

INSERT INTO comments (id, post_id, author_id, content, created_at, updated_at) VALUES
('c0000001-0000-4000-8000-000000000001', 'a1111111-1111-4111-8111-111111111111', '22222222-2222-4222-8222-222222222222',
 'The standard library routing in 1.22+ is genuinely enough for this size of service.', now() - interval '5 days', now() - interval '5 days'),
('c0000002-0000-4000-8000-000000000002', 'b1111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-111111111111',
 'The missing WHERE clause is always the plot twist.', now() - interval '4 days', now() - interval '4 days'),
('c0000003-0000-4000-8000-000000000003', 'c1111111-1111-4111-8111-111111111111', '55555555-5555-4555-8555-555555555555',
 'Please write it up. Shutdown behaviour is where most worker examples stop.', now() - interval '3 days', now() - interval '3 days'),
('c0000004-0000-4000-8000-000000000004', 'e1111111-1111-4111-8111-111111111111', '33333333-3333-4333-8333-333333333333',
 'The parameters in the PHC string are underrated. You can upgrade cost without breaking old hashes.', now() - interval '10 hours', now() - interval '10 hours');

INSERT INTO reactions (post_id, user_id, type, created_at, updated_at) VALUES
('a1111111-1111-4111-8111-111111111111', '22222222-2222-4222-8222-222222222222', 'like', now() - interval '5 days', now() - interval '5 days'),
('a1111111-1111-4111-8111-111111111111', '44444444-4444-4444-8444-444444444444', 'like', now() - interval '4 days', now() - interval '4 days'),
('a1111111-1111-4111-8111-111111111111', '55555555-5555-4555-8555-555555555555', 'love', now() - interval '3 days', now() - interval '3 days'),
('b1111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-111111111111', 'love', now() - interval '4 days', now() - interval '4 days'),
('b1111111-1111-4111-8111-111111111111', '33333333-3333-4333-8333-333333333333', 'laugh', now() - interval '3 days', now() - interval '3 days'),
('c2222222-2222-4222-8222-222222222222', '11111111-1111-4111-8111-111111111111', 'like', now() - interval '2 days', now() - interval '2 days'),
('e1111111-1111-4111-8111-111111111111', '22222222-2222-4222-8222-222222222222', 'like', now() - interval '9 hours', now() - interval '9 hours');
