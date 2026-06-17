package forum

import (
	"database/sql"
	"time"
)

type DB struct {
	db *sql.DB
}

type User struct {
	ID          int
	Username    string
	DisplayName string
	AvatarURL   string
	Gender      string
}

type Post struct {
	ID        int
	Title     string
	Content   string
	Category  string
	AuthorID  int
	Author    string
	Score     int
	CreatedAt time.Time
}

type Comment struct {
	ID        int
	PostID    int
	AuthorID  int
	Author    string
	Content   string
	Score     int
	CreatedAt time.Time
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPost(s scanner, p *Post) error {
	var created int64
	if err := s.Scan(&p.ID, &p.Title, &p.Content, &p.Category, &p.AuthorID, &p.Author, &created, &p.Score); err != nil {
		return err
	}
	p.CreatedAt = time.Unix(created, 0)
	return nil
}

func scanComment(s scanner, c *Comment) error {
	var created int64
	if err := s.Scan(&c.ID, &c.PostID, &c.Content, &c.Author, &c.AuthorID, &created, &c.Score); err != nil {
		return err
	}
	c.CreatedAt = time.Unix(created, 0)
	return nil
}

func affectedOne(res sql.Result) error {
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}
