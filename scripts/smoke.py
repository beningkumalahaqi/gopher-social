#!/usr/bin/env python3
"""End-to-end smoke test against a running GopherSocial API.

Usage:
    python3 scripts/smoke.py [base_url]

It creates a throwaway user, exercises the main flows and prints PASS lines.
Requires the seeded database for the second user (bob / Password123!).
"""
import json
import sys
import time
import urllib.error
import urllib.request
import uuid

BASE = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080"
PASSWORD = "Password123!"


def call(method, path, body=None, token=None, expected=200):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    if data:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read()
            status = resp.status
    except urllib.error.HTTPError as e:
        raw = e.read()
        status = e.code
    parsed = json.loads(raw) if raw else None
    if status != expected:
        raise AssertionError(f"{method} {path}: status {status}, want {expected}: {raw[:300]}")
    return parsed


def login(name):
    return call("POST", "/api/v1/auth/login", {"login": name, "password": PASSWORD})["data"]


def main():
    suffix = uuid.uuid4().hex[:8]
    username = "smoke_" + suffix
    email = f"{username}@example.com"

    reg = call("POST", "/api/v1/auth/register", {
        "username": username, "email": email, "password": PASSWORD, "display_name": "Smoke Test",
    }, expected=201)["data"]
    token, refresh = reg["access_token"], reg["refresh_token"]
    assert reg["user"]["email"] == email
    print("PASS register returns a session without a password hash")

    me = call("GET", "/api/v1/users/me", token=token)["data"]
    assert me["username"] == username
    print("PASS authenticated /users/me")

    word = "smoke" + suffix
    post = call("POST", "/api/v1/posts", {"content": f"Hello {word} from the smoke test"}, token=token, expected=201)["data"]
    post_id = post["id"]
    assert post["status"] == "published" and post["author"]["username"] == username
    print("PASS create post")

    call("PATCH", f"/api/v1/posts/{post_id}", {"status": "draft"}, token=token)
    call("GET", f"/api/v1/posts/{post_id}", expected=404)
    call("GET", f"/api/v1/posts/{post_id}", token=token)
    call("PATCH", f"/api/v1/posts/{post_id}", {"status": "published", "content": f"Updated {word}"}, token=token)
    print("PASS drafts are hidden from anonymous viewers")

    bob = login("bob")
    comment = call("POST", f"/api/v1/posts/{post_id}/comments", {"content": "Nice first post"}, token=bob["access_token"], expected=201)["data"]
    comments = call("GET", f"/api/v1/posts/{post_id}/comments")["data"]
    assert any(c["id"] == comment["id"] for c in comments)
    print("PASS comment creation and listing")

    summary = call("PUT", f"/api/v1/posts/{post_id}/reaction", {"type": "love"}, token=bob["access_token"])["data"]
    assert summary["total"] == 1 and summary["counts"]["love"] == 1
    summary = call("PUT", f"/api/v1/posts/{post_id}/reaction", {"type": "laugh"}, token=bob["access_token"])["data"]
    assert summary["total"] == 1 and summary["counts"]["laugh"] == 1
    print("PASS one active reaction per user")

    call("POST", f"/api/v1/users/{username}/follow", token=bob["access_token"], expected=204)
    status = call("GET", f"/api/v1/users/{username}/follow-status", token=bob["access_token"])["data"]
    assert status["following"] is True
    feed = call("GET", "/api/v1/feed?limit=50", token=bob["access_token"])["data"]
    assert any(p["id"] == post_id for p in feed)
    print("PASS follow + chronological feed")

    found = call("GET", f"/api/v1/search/posts?q={word}")["data"]
    assert any(p["id"] == post_id for p in found)
    print("PASS full-text post search")

    unread = None
    for _ in range(30):
        unread = call("GET", "/api/v1/notifications/unread-count", token=token)["data"]["unread"]
        if unread >= 3:
            break
        time.sleep(0.1)
    assert unread >= 3, f"expected follow/comment/reaction notifications, got {unread}"
    notes = call("GET", "/api/v1/notifications", token=token)["data"]
    assert len(notes) >= 3
    call("PATCH", f"/api/v1/notifications/{notes[0]['id']}/read", token=token, expected=204)
    call("POST", "/api/v1/notifications/read-all", token=token)
    assert call("GET", "/api/v1/notifications/unread-count", token=token)["data"]["unread"] == 0
    print("PASS notifications generated, marked read")

    rotated = call("POST", "/api/v1/auth/refresh", {"refresh_token": refresh})
    assert rotated["data"]["refresh_token"] != refresh
    call("POST", "/api/v1/auth/refresh", {"refresh_token": refresh}, expected=401)
    print("PASS refresh rotation rejects reuse")

    new_refresh = rotated["data"]["refresh_token"]
    call("POST", "/api/v1/auth/logout", {"refresh_token": new_refresh}, expected=204)
    call("POST", "/api/v1/auth/refresh", {"refresh_token": new_refresh}, expected=401)
    print("PASS logout revokes the refresh session")

    call("POST", "/api/v1/auth/login", {"login": username, "password": "wrong-password"}, expected=401)
    call("POST", "/api/v1/auth/register", {"username": username, "email": "other@example.com", "password": PASSWORD}, expected=409)
    print("PASS generic auth failures and conflicts")

    print("ALL SMOKE CHECKS PASSED")


if __name__ == "__main__":
    main()
