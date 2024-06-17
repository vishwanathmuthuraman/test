package routes

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"html/template"
	"net/http"
	url2 "net/url"
	"runtime/debug"
)

func Handler(db *pgxpool.Pool, funcs template.FuncMap, handler func(http.ResponseWriter, *http.Request, *pgx.Conn, template.FuncMap)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := db.Acquire(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer conn.Release()

		handler(w, r, conn.Conn(), funcs)
	}
}

func ErrorBack(w http.ResponseWriter, r *http.Request, message string, customUrl string) {
	fmt.Println("Error:", message, "URL:", r.URL)
	debug.PrintStack()
	referrer := r.Header.Get("Referer")
	if customUrl != "" {
		referrer = customUrl
	}
	referrerUrl, err := url2.Parse(referrer)
	if err != nil {
		http.Error(w, message, 500)
		return
	}
	vals := referrerUrl.Query()
	vals.Set("ui_error", message)
	vals.Del("ui_status")
	referrerUrl.RawQuery = vals.Encode()
	http.Redirect(w, r, referrerUrl.String(), 302)
}

func SuccessBack(w http.ResponseWriter, r *http.Request, message string, customUrl string) {
	referrer := r.Header.Get("Referer")
	if customUrl != "" {
		referrer = customUrl
	}
	referrerUrl, err := url2.Parse(referrer)
	if err != nil {
		http.Redirect(w, r, referrer, 302)
		return
	}
	vals := referrerUrl.Query()
	vals.Set("ui_status", message)
	vals.Del("ui_error")
	referrerUrl.RawQuery = vals.Encode()
	http.Redirect(w, r, referrerUrl.String(), 302)

}

type Login struct {
	Type     string
	Username string
	Hash     string
}

type WebUserAccount struct {
	Id       int
	Username string
	Email    string
	Type     string
}

func GetLogin(r *http.Request, db *pgx.Conn) (*WebUserAccount, error) {
	cookie, err := r.Cookie("login")
	account := WebUserAccount{}
	if err != nil {
		return nil, err
	}

	decodeString, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, err
	}

	var login Login

	err = json.NewDecoder(bytes.NewReader(decodeString)).Decode(&login)
	if err != nil {
		return nil, err
	}

	if login.Type == "va" {
		row := db.QueryRow(r.Context(), "SELECT id, name, email from va where email = $1 and password_hash = $2", login.Username, login.Hash)
		err := row.Scan(&account.Id, &account.Username, &account.Email)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errors.New("Bad username or password, please contact an admin if you have forgotten your password")
			}
			return &account, err
		}
		account.Type = "va"
		return &account, nil
	} else if login.Type == "admin" {
		row := db.QueryRow(r.Context(), "SELECT id, email from admin where email = $1 and password_hash = $2", login.Username, login.Hash)
		err := row.Scan(&account.Id, &account.Email)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errors.New("Bad username or password, please contact atulya@jaamediamarketing.com if you have forgotten your password")
			}
			return nil, err
		}
		account.Type = "admin"
		return &account, nil
	} else if login.Type == "sponsor" {
		row := db.QueryRow(r.Context(), "SELECT id, name, email from sponsor where email = $1 and password_hash = $2", login.Username, login.Hash)
		err := row.Scan(&account.Id, &account.Username, &account.Email)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errors.New("Bad username or password, please contact us if you have forgotten your password")
			}
			return nil, err
		}
		account.Type = "sponsor"
		return &account, nil
	} else if login.Type == "writer" {
		row := db.QueryRow(r.Context(), "SELECT id, name, email from writer where email = $1 and pw_hash = $2", login.Username, login.Hash)
		err := row.Scan(&account.Id, &account.Username, &account.Email)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errors.New("Bad username or password, please contact us if you have forgotten your password")
			}
			return nil, err
		}
		account.Type = "writer"
		return &account, nil
	} else {
		return nil, errors.New("invalid user type")
	}
}

type InnerData interface{}

type GlobalTemplateWrapper[T any] struct {
	InnerData      T
	Navigation     map[string][]Route
	SuccessMessage string
	ErrorMessage   string
}

func WrapGlobal[Inner any](account *WebUserAccount, successMessage string, errorMessage string, innerData Inner) GlobalTemplateWrapper[Inner] {
	return GlobalTemplateWrapper[Inner]{
		Navigation:     RbacNavigation(account),
		InnerData:      innerData,
		SuccessMessage: successMessage,
		ErrorMessage:   errorMessage,
	}
}
