package src

import (
	"log"
	"net/http"
)

// Login creates a session token for the user and logs the login.
func Login(w http.ResponseWriter, user string) {
	token := newToken(user)
	log.Printf("login ok user=%s token=%s", user, token)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, HttpOnly: true})
}

func newToken(user string) string {
	return "tok-" + user
}
