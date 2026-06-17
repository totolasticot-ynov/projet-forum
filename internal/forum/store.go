package forum

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
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

func NewDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_foreign_keys=ON")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	d := &DB{db: db}
	if err := d.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) initSchema() error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, username TEXT UNIQUE NOT NULL, password_hash TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, user_id INTEGER NOT NULL, expires_at INTEGER NOT NULL, FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);`,
		`CREATE TABLE IF NOT EXISTS posts (id INTEGER PRIMARY KEY, author_id INTEGER NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL, category TEXT NOT NULL, created_at INTEGER NOT NULL, FOREIGN KEY(author_id) REFERENCES users(id) ON DELETE CASCADE);`,
		`CREATE TABLE IF NOT EXISTS comments (id INTEGER PRIMARY KEY, post_id INTEGER NOT NULL, author_id INTEGER NOT NULL, content TEXT NOT NULL, created_at INTEGER NOT NULL, FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE, FOREIGN KEY(author_id) REFERENCES users(id) ON DELETE CASCADE);`,
		`CREATE TABLE IF NOT EXISTS post_votes (post_id INTEGER NOT NULL, user_id INTEGER NOT NULL, value INTEGER NOT NULL, PRIMARY KEY(post_id,user_id), FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE, FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);`,
		`CREATE TABLE IF NOT EXISTS comment_votes (comment_id INTEGER NOT NULL, user_id INTEGER NOT NULL, value INTEGER NOT NULL, PRIMARY KEY(comment_id,user_id), FOREIGN KEY(comment_id) REFERENCES comments(id) ON DELETE CASCADE, FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);`,
	}
	for _, stmt := range schema {
		if _, err := d.db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := d.ensureUserColumns(); err != nil {
		return err
	}
	return nil
}

func (d *DB) ensureUserColumns() error {
	rows, err := d.db.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}

	if !existing["display_name"] {
		if _, err := d.db.Exec(`ALTER TABLE users ADD COLUMN display_name TEXT`); err != nil {
			return err
		}
	}
	if !existing["avatar_url"] {
		if _, err := d.db.Exec(`ALTER TABLE users ADD COLUMN avatar_url TEXT`); err != nil {
			return err
		}
	}
	if !existing["gender"] {
		if _, err := d.db.Exec(`ALTER TABLE users ADD COLUMN gender TEXT`); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Register(username, password string) error {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return errors.New("nom d'utilisateur et mot de passe requis")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`INSERT INTO users(username,password_hash,display_name,avatar_url,gender) VALUES(?,?,?,?,?)`, username, string(hash), username, "", "")
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return errors.New("ce nom d'utilisateur est déjà pris")
		}
		return err
	}
	return nil
}

func (d *DB) Authenticate(username, password string) (*User, error) {
	row := d.db.QueryRow(`SELECT id, password_hash FROM users WHERE username = ?`, username)
	var id int
	var hash string
	if err := row.Scan(&id, &hash); err != nil {
		return nil, errors.New("identifiants invalides")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, errors.New("identifiants invalides")
	}
	return &User{ID: id, Username: username}, nil
}

func (d *DB) CreateSession(userID int) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(30 * 24 * time.Hour).Unix()
	_, err = d.db.Exec(`INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)`, token, userID, expires)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (d *DB) UserBySession(token string) (*User, error) {
	row := d.db.QueryRow(`SELECT u.id, u.username, COALESCE(u.display_name, u.username), COALESCE(u.avatar_url, ''), COALESCE(u.gender, '') FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token = ? AND s.expires_at > ?`, token, time.Now().Unix())
	var user User
	if err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL, &user.Gender); err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *DB) DeleteSession(token string) {
	_, _ = d.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
}

func (d *DB) UpdateUser(userID int, username, displayName, avatarURL, gender string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("nom d'utilisateur requis")
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = username
	}
	res, err := d.db.Exec(`UPDATE users SET username = ?, display_name = ?, avatar_url = ?, gender = ? WHERE id = ?`, username, displayName, avatarURL, gender, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return errors.New("ce nom d'utilisateur est déjà pris")
		}
		return err
	}
	if count, err := res.RowsAffected(); err == nil && count == 0 {
		return errors.New("utilisateur introuvable")
	}
	return nil
}

func (d *DB) CreatePost(userID int, title, content, category string) (int, error) {
	now := time.Now().Unix()
	res, err := d.db.Exec(`INSERT INTO posts(author_id,title,content,category,created_at) VALUES(?,?,?,?,?)`, userID, title, content, category, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (d *DB) ListPosts(category string) ([]*Post, error) {
	var rows *sql.Rows
	var err error
	base := `SELECT p.id, p.title, p.content, p.category, p.author_id, COALESCE(u.display_name, u.username), p.created_at, COALESCE((SELECT SUM(value) FROM post_votes v WHERE v.post_id = p.id),0) FROM posts p JOIN users u ON u.id = p.author_id`
	if category == "" {
		rows, err = d.db.Query(base + ` ORDER BY p.created_at DESC`)
	} else {
		rows, err = d.db.Query(base+` WHERE p.category = ? ORDER BY p.created_at DESC`, category)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := []*Post{}
	for rows.Next() {
		p := &Post{}
		var created int64
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.Category, &p.AuthorID, &p.Author, &created, &p.Score); err != nil {
			return nil, err
		}
		p.CreatedAt = time.Unix(created, 0)
		posts = append(posts, p)
	}
	return posts, nil
}

func (d *DB) GetPost(id int) (*Post, error) {
	row := d.db.QueryRow(`SELECT p.id, p.title, p.content, p.category, p.author_id, COALESCE(u.display_name, u.username), p.created_at, COALESCE((SELECT SUM(value) FROM post_votes v WHERE v.post_id = p.id),0) FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id = ?`, id)
	p := &Post{}
	var created int64
	if err := row.Scan(&p.ID, &p.Title, &p.Content, &p.Category, &p.AuthorID, &p.Author, &created, &p.Score); err != nil {
		return nil, err
	}
	p.CreatedAt = time.Unix(created, 0)
	return p, nil
}

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
		var created int64
		if err := rows.Scan(&c.ID, &c.PostID, &c.Content, &c.Author, &c.AuthorID, &created, &c.Score); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(created, 0)
		comments = append(comments, c)
	}
	return comments, nil
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

func (d *DB) UpdatePost(postID, userID int, title, content, category string) error {
	res, err := d.db.Exec(`UPDATE posts SET title = ?, content = ?, category = ? WHERE id = ? AND author_id = ?`, title, content, category, postID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *DB) DeletePost(postID, userID int) error {
	res, err := d.db.Exec(`DELETE FROM posts WHERE id = ? AND author_id = ?`, postID, userID)
	if err != nil {
		return err
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *DB) UpdateComment(commentID, userID int, content string) error {
	res, err := d.db.Exec(`UPDATE comments SET content = ? WHERE id = ? AND author_id = ?`, content, commentID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *DB) DeleteComment(commentID, userID int) error {
	res, err := d.db.Exec(`DELETE FROM comments WHERE id = ? AND author_id = ?`, commentID, userID)
	if err != nil {
		return err
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
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

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
