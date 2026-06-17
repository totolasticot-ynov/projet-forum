package forum

import (
	"strings"
	"time"
)

func (d *DB) CreatePost(userID int, title, content, category string) (int, error) {
	now := time.Now().Unix()
	res, err := d.db.Exec(`INSERT INTO posts(author_id,title,content,category,created_at) VALUES(?,?,?,?,?)`, userID, title, content, category, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (d *DB) ListPosts(category, search string) ([]*Post, error) {
	base := `SELECT p.id, p.title, p.content, p.category, p.author_id, COALESCE(u.display_name, u.username), p.created_at, COALESCE((SELECT SUM(value) FROM post_votes v WHERE v.post_id = p.id),0) FROM posts p JOIN users u ON u.id = p.author_id`
	where, args := []string{}, []any{}
	if category != "" {
		where = append(where, `p.category = ?`)
		args = append(args, category)
	}
	if search != "" {
		where = append(where, `(LOWER(p.title) LIKE ? OR LOWER(p.content) LIKE ?)`)
		args = append(args, "%"+strings.ToLower(search)+"%", "%"+strings.ToLower(search)+"%")
	}
	if len(where) > 0 {
		base += ` WHERE ` + strings.Join(where, ` AND `)
	}
	base += ` ORDER BY p.created_at DESC`

	rows, err := d.db.Query(base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []*Post{}
	for rows.Next() {
		p := &Post{}
		if err := scanPost(rows, p); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (d *DB) GetPost(id int) (*Post, error) {
	row := d.db.QueryRow(`SELECT p.id, p.title, p.content, p.category, p.author_id, COALESCE(u.display_name, u.username), p.created_at, COALESCE((SELECT SUM(value) FROM post_votes v WHERE v.post_id = p.id),0) FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id = ?`, id)
	p := &Post{}
	if err := scanPost(row, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (d *DB) UpdatePost(postID, userID int, title, content, category string) error {
	res, err := d.db.Exec(`UPDATE posts SET title = ?, content = ?, category = ? WHERE id = ? AND author_id = ?`, title, content, category, postID, userID)
	if err != nil {
		return err
	}
	return affectedOne(res)
}

func (d *DB) DeletePost(postID, userID int) error {
	res, err := d.db.Exec(`DELETE FROM posts WHERE id = ? AND author_id = ?`, postID, userID)
	if err != nil {
		return err
	}
	return affectedOne(res)
}
