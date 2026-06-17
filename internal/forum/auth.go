package forum

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

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

func (d *DB) UserStats(userID int) (posts, comments int, err error) {
	if err = d.db.QueryRow(`SELECT COUNT(*) FROM posts WHERE author_id = ?`, userID).Scan(&posts); err != nil {
		return 0, 0, err
	}
	if err = d.db.QueryRow(`SELECT COUNT(*) FROM comments WHERE author_id = ?`, userID).Scan(&comments); err != nil {
		return 0, 0, err
	}
	return posts, comments, nil
}
