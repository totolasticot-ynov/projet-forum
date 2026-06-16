package web

import (
    "embed"
    "html/template"
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
        posts, _ := db.ListPosts(category)
        render(w, "home", map[string]interface{}{
            "Title":      "Forum Go",
            "User":       user,
            "Posts":      posts,
            "Category":   category,
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
