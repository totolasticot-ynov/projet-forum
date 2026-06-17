package forum

import "time"

func (d *DB) CreateComment(postID, userID int, content string) error {
	now := time.Now().Unix()
	_, err := d.db.Exec(`INSERT INTO comments(post_id,author_id,content,created_at) VALUES(?,?,?,?)`, postID, userID, content, now)
	return err
}

func (d *DB) ListComments(postID int) ([]*Comment, error) {
	rows, err := d.db.Query(`SELECT c.id, c.post_id, c.content, COALESCE(u.display_name, u.username), c.author_id, c.created_at, COALESCE((SELECT SUM(value) FROM comment_votes v WHERE v.comment_id = c.id),0) FROM comments c JOIN users u ON u.id = c.author_id WHERE c.post_id = ? ORDER BY c.created_at`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []*Comment{}
	for rows.Next() {
		c := &Comment{}
		if err := scanComment(rows, c); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (d *DB) GetComment(id int) (*Comment, error) {
	row := d.db.QueryRow(`SELECT c.id, c.post_id, c.author_id, c.content, COALESCE(u.display_name, u.username), c.created_at FROM comments c JOIN users u ON u.id = c.author_id WHERE c.id = ?`, id)
	c := &Comment{}
	var created int64
	if err := row.Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.Author, &created); err != nil {
		return nil, err
	}
	c.CreatedAt = time.Unix(created, 0)
	return c, nil
}

func (d *DB) UpdateComment(commentID, userID int, content string) error {
	res, err := d.db.Exec(`UPDATE comments SET content = ? WHERE id = ? AND author_id = ?`, content, commentID, userID)
	if err != nil {
		return err
	}
	return affectedOne(res)
}

func (d *DB) DeleteComment(commentID, userID int) error {
	res, err := d.db.Exec(`DELETE FROM comments WHERE id = ? AND author_id = ?`, commentID, userID)
	if err != nil {
		return err
	}
	return affectedOne(res)
}

func (d *DB) VotePost(userID, postID, value int) error {
	if value != 1 && value != -1 {
		return nil
	}
	current := 0
	row := d.db.QueryRow(`SELECT value FROM post_votes WHERE post_id = ? AND user_id = ?`, postID, userID)
	_ = row.Scan(&current)
	if current == value {
		_, err := d.db.Exec(`DELETE FROM post_votes WHERE post_id = ? AND user_id = ?`, postID, userID)
		return err
	}
	_, err := d.db.Exec(`INSERT INTO post_votes(post_id,user_id,value) VALUES(?,?,?) ON CONFLICT(post_id,user_id) DO UPDATE SET value=excluded.value`, postID, userID, value)
	return err
}

func (d *DB) VoteComment(userID, commentID, value int) error {
	if value != 1 && value != -1 {
		return nil
	}
	current := 0
	row := d.db.QueryRow(`SELECT value FROM comment_votes WHERE comment_id = ? AND user_id = ?`, commentID, userID)
	_ = row.Scan(&current)
	if current == value {
		_, err := d.db.Exec(`DELETE FROM comment_votes WHERE comment_id = ? AND user_id = ?`, commentID, userID)
		return err
	}
	_, err := d.db.Exec(`INSERT INTO comment_votes(comment_id,user_id,value) VALUES(?,?,?) ON CONFLICT(comment_id,user_id) DO UPDATE SET value=excluded.value`, commentID, userID, value)
	return err
}
