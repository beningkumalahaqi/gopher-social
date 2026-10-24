// Package dto defines the JSON shapes of the HTTP API. Domain models never
// carry JSON tags, so every field is a deliberate inclusion.
package dto

import (
	"time"

	"github.com/haqi/gophersocial/internal/auth"
	"github.com/haqi/gophersocial/internal/comments"
	"github.com/haqi/gophersocial/internal/notifications"
	"github.com/haqi/gophersocial/internal/posts"
	"github.com/haqi/gophersocial/internal/reactions"
	"github.com/haqi/gophersocial/internal/users"
)

type UserSummary struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	AvatarURL   *string   `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type PrivateUser struct {
	User
	Email string `json:"email"`
}

type UserStats struct {
	Posts     int `json:"posts"`
	Followers int `json:"followers"`
	Following int `json:"following"`
}

type Profile struct {
	User
	Stats UserStats `json:"stats"`
}

type Session struct {
	User             PrivateUser `json:"user"`
	TokenType        string      `json:"token_type"`
	AccessToken      string      `json:"access_token"`
	RefreshToken     string      `json:"refresh_token"`
	ExpiresIn        int         `json:"expires_in"`
	RefreshExpiresAt time.Time   `json:"refresh_expires_at"`
}

type Post struct {
	ID             string      `json:"id"`
	Content        string      `json:"content"`
	ImageURL       *string     `json:"image_url"`
	Status         string      `json:"status"`
	Author         UserSummary `json:"author"`
	CommentCount   int         `json:"comment_count"`
	ReactionCount  int         `json:"reaction_count"`
	ViewerReaction *string     `json:"viewer_reaction"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	PublishedAt    *time.Time  `json:"published_at"`
}

type Comment struct {
	ID        string      `json:"id"`
	PostID    string      `json:"post_id"`
	Content   string      `json:"content"`
	Author    UserSummary `json:"author"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type Reaction struct {
	PostID    string      `json:"post_id"`
	Type      string      `json:"type"`
	User      UserSummary `json:"user"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type ReactionSummary struct {
	Total  int            `json:"total"`
	Counts map[string]int `json:"counts"`
	Viewer *string        `json:"viewer_reaction"`
}

type Notification struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Actor     UserSummary `json:"actor"`
	PostID    *string     `json:"post_id"`
	CommentID *string     `json:"comment_id"`
	ReadAt    *time.Time  `json:"read_at"`
	CreatedAt time.Time   `json:"created_at"`
}

func NewUser(u users.Public) User {
	return User{
		ID:          u.ID.String(),
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Bio:         u.Bio,
		AvatarURL:   u.AvatarURL,
		CreatedAt:   u.CreatedAt,
	}
}

func NewPrivateUser(u users.User) PrivateUser {
	return PrivateUser{User: NewUser(users.Public{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Bio:         u.Bio,
		AvatarURL:   u.AvatarURL,
		CreatedAt:   u.CreatedAt,
	}), Email: u.Email}
}

func NewProfile(p users.Profile) Profile {
	return Profile{
		User: NewUser(p.User),
		Stats: UserStats{
			Posts:     p.Stats.Posts,
			Followers: p.Stats.Followers,
			Following: p.Stats.Following,
		},
	}
}

func NewSession(s auth.Session) Session {
	expiresIn := int(time.Until(s.AccessExpiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}
	return Session{
		User:             NewPrivateUser(s.User),
		TokenType:        "Bearer",
		AccessToken:      s.AccessToken,
		RefreshToken:     s.RefreshToken,
		ExpiresIn:        expiresIn,
		RefreshExpiresAt: s.RefreshExpiresAt,
	}
}

func NewPost(p posts.Post) Post {
	return Post{
		ID:       p.ID.String(),
		Content:  p.Content,
		ImageURL: p.ImageURL,
		Status:   string(p.Status),
		Author: UserSummary{
			ID:          p.Author.ID.String(),
			Username:    p.Author.Username,
			DisplayName: p.Author.DisplayName,
			AvatarURL:   p.Author.AvatarURL,
		},
		CommentCount:   p.CommentCount,
		ReactionCount:  p.ReactionCount,
		ViewerReaction: p.ViewerReaction,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		PublishedAt:    p.PublishedAt,
	}
}

func NewPosts(items []posts.Post) []Post {
	out := make([]Post, 0, len(items))
	for _, p := range items {
		out = append(out, NewPost(p))
	}
	return out
}

func NewComment(c comments.Comment) Comment {
	return Comment{
		ID:      c.ID.String(),
		PostID:  c.PostID.String(),
		Content: c.Content,
		Author: UserSummary{
			ID:          c.Author.ID.String(),
			Username:    c.Author.Username,
			DisplayName: c.Author.DisplayName,
			AvatarURL:   c.Author.AvatarURL,
		},
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func NewComments(items []comments.Comment) []Comment {
	out := make([]Comment, 0, len(items))
	for _, c := range items {
		out = append(out, NewComment(c))
	}
	return out
}

func NewReaction(r reactions.Reaction) Reaction {
	return Reaction{
		PostID: r.PostID.String(),
		Type:   string(r.Type),
		User: UserSummary{
			Username:    r.User.Username,
			DisplayName: r.User.DisplayName,
			AvatarURL:   r.User.AvatarURL,
		},
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func NewReactions(items []reactions.Reaction) []Reaction {
	out := make([]Reaction, 0, len(items))
	for _, r := range items {
		out = append(out, NewReaction(r))
	}
	return out
}

func NewReactionSummary(s reactions.Summary) ReactionSummary {
	counts := make(map[string]int, len(s.Counts))
	for k, v := range s.Counts {
		counts[string(k)] = v
	}
	var viewer *string
	if s.Viewer != nil {
		v := string(*s.Viewer)
		viewer = &v
	}
	return ReactionSummary{Total: s.Total, Counts: counts, Viewer: viewer}
}

func NewNotification(n notifications.Notification) Notification {
	var postID, commentID *string
	if n.PostID != nil {
		v := n.PostID.String()
		postID = &v
	}
	if n.CommentID != nil {
		v := n.CommentID.String()
		commentID = &v
	}
	return Notification{
		ID:   n.ID.String(),
		Type: string(n.Type),
		Actor: UserSummary{
			ID:          n.ActorID.String(),
			Username:    n.Actor.Username,
			DisplayName: n.Actor.DisplayName,
			AvatarURL:   n.Actor.AvatarURL,
		},
		PostID:    postID,
		CommentID: commentID,
		ReadAt:    n.ReadAt,
		CreatedAt: n.CreatedAt,
	}
}

func NewNotifications(items []notifications.Notification) []Notification {
	out := make([]Notification, 0, len(items))
	for _, n := range items {
		out = append(out, NewNotification(n))
	}
	return out
}
