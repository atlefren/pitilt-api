package main

import (
	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/context"
	"net/http"
)

type KeyCheckHandler struct {
	db *Database
}

type JwtCheckHandler struct {
	db            *Database
	jwtMiddleware *jwtmiddleware.JWTMiddleware
}

func (h *KeyCheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	key := r.Header.Get("X-PYTILT-KEY")
	if key == "" {
		http.Error(w, "key not specified", http.StatusUnauthorized)
		return
	}
	user, err := h.db.getUserForKey(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
	}
	if err == nil && next != nil {
		context.Set(r, "user", user)
		next(w, r)
	}
}

func (h *JwtCheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	err := h.jwtMiddleware.CheckJWT(w, r)
	if err == nil && next != nil {
		claims := context.Get(r, "user").(*jwt.Token).Claims.(jwt.MapClaims)
		if claims["iss"] == "https://securetoken.google.com/pitilt-7a37c" && claims["aud"] == "pitilt-7a37c" {
			userId := claims["user_id"].(string)
			exists, err := h.db.userExists(userId)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			if !exists {
				h.db.createUser(userId, claims["email"].(string), claims["name"].(string))
			}
			context.Set(r, "user", userId)
			next(w, r)
		} else {
			http.Error(w, "key not valid", http.StatusUnauthorized)
		}
	}
}
