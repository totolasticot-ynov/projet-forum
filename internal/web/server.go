package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"forum/internal/forum"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/styles.css
var stylesCSS string

var tpl = template.Must(template.ParseFS(templatesFS, "templates/*.html"))

var categories = []string{"Général", "Tech", "Aide", "Annonce"}

func New() http.Handler {
	db, err := forum.NewDB("forum.db")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r, db)
		category := strings.TrimSpace(r.URL.Query().Get("category"))
		search := strings.TrimSpace(r.URL.Query().Get("q"))
		posts, _ := db.ListPosts(category, search)
		render(w, "home", map[string]interface{}{
			"Title":      "Forum Go",
			"User":       user,
			"Posts":      posts,
			"Category":   category,
			"Search":     search,
			"Categories": categories,
		})
	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r, db) != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		data := map[string]interface{}{"Title": "Connexion"}
		if r.Method == http.MethodPost {
			err := r.ParseForm()
			if err == nil {
				user, err := db.Authenticate(r.FormValue("username"), r.FormValue("password"))
				if err == nil {
					token, err := db.CreateSession(user.ID)
					if err == nil {
						http.SetCookie(w, &http.Cookie{Name: "forum_session", Value: token, Path: "/", HttpOnly: true})
						http.Redirect(w, r, "/", http.StatusSeeOther)
						return
					}
				} else {
					data["Flash"] = err.Error()
				}
			}
		}
		render(w, "login", data)
	})

	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r, db) != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		data := map[string]interface{}{"Title": "Inscription"}
		if r.Method == http.MethodPost {
			err := r.ParseForm()
			if err == nil {
				username := strings.TrimSpace(r.FormValue("username"))
				password := r.FormValue("password")
				err = db.Register(username, password)
				if err == nil {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
				data["Flash"] = err.Error()
			}
		}
		render(w, "register", data)
	})

	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r, db)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		postsCount, commentsCount, statsErr := db.UserStats(user.ID)
		data := map[string]interface{}{
			"Title":         "Mon profil",
			"User":          user,
			"PostsCount":    postsCount,
			"CommentsCount": commentsCount,
		}
		if statsErr != nil {
			data["Flash"] = "Impossible de charger les statistiques"
		}
		if r.Method == http.MethodPost {
			err := r.ParseForm()
			if err == nil {
				username := strings.TrimSpace(r.FormValue("username"))
				displayName := strings.TrimSpace(r.FormValue("display_name"))
				avatarURL := strings.TrimSpace(r.FormValue("avatar_url"))
				gender := strings.TrimSpace(r.FormValue("gender"))
				err = db.UpdateUser(user.ID, username, displayName, avatarURL, gender)
				if err == nil {
					cookie, cookieErr := r.Cookie("forum_session")
					if cookieErr == nil {
						if updatedUser, updatedErr := db.UserBySession(cookie.Value); updatedErr == nil {
							user = updatedUser
						}
					}
					data["User"] = user
					postsCount, commentsCount, statsErr = db.UserStats(user.ID)
					if statsErr == nil {
						data["PostsCount"] = postsCount
						data["CommentsCount"] = commentsCount
					}
					http.Redirect(w, r, "/profile", http.StatusSeeOther)
					return
				}
				data["Flash"] = err.Error()
			}
		}
		render(w, "profile", data)
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("forum_session")
		if err == nil {
			db.DeleteSession(cookie.Value)
			http.SetCookie(w, &http.Cookie{Name: "forum_session", Value: "", Path: "/", MaxAge: -1})
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	mux.HandleFunc("/post/new", func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r, db)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			title := strings.TrimSpace(r.FormValue("title"))
			category := strings.TrimSpace(r.FormValue("category"))
			content := strings.TrimSpace(r.FormValue("content"))
			if title != "" && content != "" && category != "" {
				id, err := db.CreatePost(user.ID, title, content, category)
				if err == nil {
					http.Redirect(w, r, "/post/"+strconv.Itoa(id), http.StatusSeeOther)
					return
				}
			}
		}
		render(w, "newpost", map[string]interface{}{
			"Title":      "Nouveau sujet",
			"User":       user,
			"Categories": categories,
		})
	})

	mux.HandleFunc("/comment/", func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r, db)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		pathID := strings.TrimPrefix(r.URL.Path, "/comment/")
		if strings.HasSuffix(pathID, "/edit") {
			commentID, err := strconv.Atoi(strings.TrimSuffix(pathID, "/edit"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			comment, err := db.GetComment(commentID)
			if err != nil || comment.AuthorID != user.ID {
				http.NotFound(w, r)
				return
			}
			if r.Method == http.MethodPost {
				_ = r.ParseForm()
				content := strings.TrimSpace(r.FormValue("content"))
				if content != "" {
					_ = db.UpdateComment(commentID, user.ID, content)
				}
				http.Redirect(w, r, "/post/"+strconv.Itoa(comment.PostID), http.StatusSeeOther)
				return
			}
			render(w, "editcomment", map[string]interface{}{
				"Title":   "Modifier le commentaire",
				"User":    user,
				"Comment": comment,
			})
			return
		}

		if strings.HasSuffix(pathID, "/delete") {
			commentID, err := strconv.Atoi(strings.TrimSuffix(pathID, "/delete"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			comment, err := db.GetComment(commentID)
			if err != nil || comment.AuthorID != user.ID {
				http.NotFound(w, r)
				return
			}
			_ = db.DeleteComment(commentID, user.ID)
			http.Redirect(w, r, "/post/"+strconv.Itoa(comment.PostID), http.StatusSeeOther)
			return
		}

		http.NotFound(w, r)
	})

	mux.HandleFunc("/post/", func(w http.ResponseWriter, r *http.Request) {
		pathID := strings.TrimPrefix(r.URL.Path, "/post/")
		if pathID == "" {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(pathID, "/comment") {
			user := currentUser(r, db)
			if user == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			postID, err := strconv.Atoi(strings.TrimSuffix(pathID, "/comment"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			_ = r.ParseForm()
			content := strings.TrimSpace(r.FormValue("content"))
			if content != "" {
				_ = db.CreateComment(postID, user.ID, content)
			}
			http.Redirect(w, r, "/post/"+strconv.Itoa(postID), http.StatusSeeOther)
			return
		}
		if strings.HasSuffix(pathID, "/vote") {
			user := currentUser(r, db)
			if user == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			postID, err := strconv.Atoi(strings.TrimSuffix(pathID, "/vote"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			_ = r.ParseForm()
			action := r.FormValue("action")
			value := 0
			if action == "like" {
				value = 1
			}
			if action == "dislike" {
				value = -1
			}
			if value != 0 {
				_ = db.VotePost(user.ID, postID, value)
			}
			http.Redirect(w, r, "/post/"+strconv.Itoa(postID), http.StatusSeeOther)
			return
		}
		if strings.HasSuffix(pathID, "/edit") {
			user := currentUser(r, db)
			if user == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			postID, err := strconv.Atoi(strings.TrimSuffix(pathID, "/edit"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			post, err := db.GetPost(postID)
			if err != nil || post.AuthorID != user.ID {
				http.NotFound(w, r)
				return
			}
			if r.Method == http.MethodPost {
				_ = r.ParseForm()
				title := strings.TrimSpace(r.FormValue("title"))
				category := strings.TrimSpace(r.FormValue("category"))
				content := strings.TrimSpace(r.FormValue("content"))
				if title != "" && content != "" && category != "" {
					_ = db.UpdatePost(postID, user.ID, title, content, category)
				}
				http.Redirect(w, r, "/post/"+strconv.Itoa(postID), http.StatusSeeOther)
				return
			}
			render(w, "editpost", map[string]interface{}{
				"Title":      "Modifier le sujet",
				"User":       user,
				"Post":       post,
				"Categories": categories,
			})
			return
		}
		if strings.HasSuffix(pathID, "/delete") {
			user := currentUser(r, db)
			if user == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			postID, err := strconv.Atoi(strings.TrimSuffix(pathID, "/delete"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			_ = db.DeletePost(postID, user.ID)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		postID, err := strconv.Atoi(pathID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		post, err := db.GetPost(postID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		comments, _ := db.ListComments(postID)
		render(w, "post", map[string]interface{}{
			"Title":    post.Title,
			"User":     currentUser(r, db),
			"Post":     post,
			"Comments": comments,
		})
	})

	mux.HandleFunc("/static/styles.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(stylesCSS))
	})

	return mux
}

func render(w http.ResponseWriter, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Erreur interne", http.StatusInternalServerError)
		log.Println("template error:", err)
	}
}

func currentUser(r *http.Request, db *forum.DB) *forum.User {
	cookie, err := r.Cookie("forum_session")
	if err != nil {
		return nil
	}
	user, _ := db.UserBySession(cookie.Value)
	return user
}
