package forum

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

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

func (d *DB) Close() error {
	return d.db.Close()
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
	return d.seedSampleData()
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
	for _, col := range []string{"display_name", "avatar_url", "gender"} {
		if existing[col] {
			continue
		}
		if _, err := d.db.Exec(`ALTER TABLE users ADD COLUMN ` + col + ` TEXT`); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) seedSampleData() error {
	var postCount int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&postCount); err != nil {
		return err
	}
	if postCount > 0 {
		return nil
	}

	var userID int
	err := d.db.QueryRow(`SELECT id FROM users WHERE username = ?`, "demo").Scan(&userID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte("demo123"), bcrypt.DefaultCost)
		if hashErr != nil {
			return hashErr
		}
		res, execErr := d.db.Exec(`INSERT INTO users(username, password_hash, display_name, avatar_url, gender) VALUES(?, ?, ?, ?, ?)`, "demo", string(passwordHash), "Démo", "", "Autre")
		if execErr != nil {
			return execErr
		}
		userID64, idErr := res.LastInsertId()
		if idErr != nil {
			return idErr
		}
		userID = int(userID64)
	}

	samplePosts := []struct {
		title    string
		content  string
		category string
	}{
		{title: "Bienvenue sur le forum", content: "Présentez-vous, dites ce qui vous intéresse et partagez vos premiers échanges avec la communauté.", category: "Général"},
		{title: "Quel outil pour développer en Go ?", content: "Quels IDE, extensions ou outils de debugging recommandez-vous pour travailler efficacement avec Go ?", category: "Tech"},
		{title: "Comment résoudre un problème de login ?", content: "Expliquez les erreurs que vous rencontrez lors de la connexion et les étapes que vous avez déjà essayées.", category: "Aide"},
		{title: "Annonce importante du projet", content: "Voici les prochaines étapes du forum, les nouveautés prévues et les dates de mise à jour.", category: "Annonce"},
	}
	now := time.Now().Unix()
	for _, post := range samplePosts {
		if _, err := d.db.Exec(`INSERT INTO posts(author_id, title, content, category, created_at) VALUES(?, ?, ?, ?, ?)`, userID, post.title, post.content, post.category, now); err != nil {
			return err
		}
	}
	return nil
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
