package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func getToken(res http.ResponseWriter, req *http.Request) {
	login := &login{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(login)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	account, err := store.Accounts.Get(login.Email, login.Password)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	newlogin := NewLogin(account)

	err = store.Logins.Insert(newlogin)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	res.Write([]byte(newlogin.Token + "\n"))
}

func createAccount(res http.ResponseWriter, req *http.Request) {
	var err error

	var acc = &struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}

	decoder := json.NewDecoder(req.Body)
	err = decoder.Decode(acc)

	if err != nil {
		res.Write([]byte(err.Error()))
		return
	}

	account, err := NewAccount(acc.Email, acc.Password)

	if err != nil {
		return
	}

	err = store.Accounts.Insert(account)

	if err != nil {
		res.Write([]byte("error: email already in use"))
		fmt.Println(err.Error())
		return
	}

	login := NewLogin(account)
	err = store.Logins.Insert(login)

	if err != nil {
		return
	}

	res.Write([]byte(login.Token + "\n"))
}

func removeBookmark(res http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)
	name := vars["name"]

	token := req.Header.Get("Auth")
	account, err := store.Logins.GetAccount(token)

	if err != nil {
		fmt.Println(err)
		return
	}

	if account == nil {
		res.Write([]byte("no account found"))
		return
	}

	bookmark := &Bookmark{
		Account: account.Id,
		Name:    name,
	}

	err = store.Bookmarks.Remove(bookmark)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	res.WriteHeader(200)
}

func createBookmark(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	paste := vars["paste"]
	name := vars["name"]

	token := req.Header.Get("Auth")

	account, err := store.Logins.GetAccount(token)

	if err != nil {
		fmt.Println(err)
		return
	}

	if account == nil {
		res.Write([]byte("no account found"))
		return
	}

	bookmark := &Bookmark{
		Account: account.Id,
		Name:    name,
		Paste:   paste,
	}

	err = store.Bookmarks.Insert(bookmark)

	if err != nil {
		res.Write([]byte(err.Error()))
		return
	}

	res.WriteHeader(200)
}

func getHistory(res http.ResponseWriter, req *http.Request) {
	token := req.Header.Get("Auth")

	if token == "" {
		res.Write([]byte("missing Auth header"))
		return
	}

	account, err := store.Logins.GetAccount(token)

	if err != nil {
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]

	bookmark := &Bookmark{
		Name:    name,
		Account: account.Id,
	}

	pastes := store.Bookmarks.GetHistory(bookmark)

	now := time.Now()
	for _, paste := range pastes {
		created := time.Unix(paste.Time/1000, 0)
		elapsed := human_time(created, now)
		fmt.Fprintf(res, "%s\t%s\n", paste.Id, elapsed)
	}
}

func human_time(t0, t1 time.Time) string {
	h := int(t1.Sub(t0).Hours())
	if h >= 48 {
		d := int(h / 24)
		return strconv.Itoa(d) + " days ago"
	} else if h >= 24 {
		return "yesterday"
	} else {
		return strconv.Itoa(h) + " hours ago"
	}
}

func getBookmark(res http.ResponseWriter, req *http.Request) {

	token := req.Header.Get("Auth")

	if token == "" {
		res.Write([]byte("missing Auth header"))
		return
	}

	account, err := store.Logins.GetAccount(token)

	if err != nil {
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]

	bookmark := &Bookmark{
		Name:    name,
		Account: account.Id,
	}

	paste := store.Bookmarks.GetPaste(bookmark)

	if paste == nil {
		return
	}

	res.Write(paste.Content)
}

func getBookmarks(res http.ResponseWriter, req *http.Request) {

	token := req.Header.Get("Auth")

	if token == "" {
		res.Write([]byte("missing Auth header"))
		return
	}

	account, err := store.Logins.GetAccount(token)

	if err != nil {
		fmt.Println(err)
		return
	}

	bookmarks, err := store.Bookmarks.Get(account)

	if err != nil {
		fmt.Println(err)
		return
	}

	maxLength := 0

	for _, bookmark := range bookmarks {
		if len(bookmark.Name) > maxLength {
			maxLength = len(bookmark.Name)
		}
	}

	maxLength += 2

	now := time.Now()
	for _, bookmark := range bookmarks {
		created := time.Unix(bookmark.Time/1000, 0)
		elapsed := human_time(created, now)
		tab := strings.Repeat(" ", maxLength-len(bookmark.Name))
		fmt.Fprintf(res, "%s%s%s\t%s\n", bookmark.Name, tab, bookmark.Paste, elapsed)
	}
}

func getPaste(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["paste"]

	paste, err := store.Pastes.Get(id)

	if err != nil {
		fmt.Println(err)
		res.WriteHeader(500)
		return
	}

	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	res.Write(paste.Content)
}

const home = `<!DOCTYPE html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<pre>cat main.go | curl --data-binary @- https://gdf3.com</pre>
<div style="position:absolute; bottom:100px;"> cli available at
<a href="https://github.com/slofurno/gdfbin">github.com/slofurno/gdfbin</a></div>`

func getHome(res http.ResponseWriter, req *http.Request) {

	res.Write([]byte(home))
	return
}

func postPaste(res http.ResponseWriter, req *http.Request) {

	buf := bytes.NewBuffer(nil)
	_, err := io.Copy(buf, req.Body)

	if err != nil {
		fmt.Println(err)
		res.WriteHeader(500)
		return
	}

	paste := NewPaste()
	paste.Content = buf.Bytes()
	err = store.Pastes.Insert(paste)

	if err != nil {
		fmt.Println(err)
		res.WriteHeader(500)
		return
	}

	res.Write([]byte("https://gdf3.com/" + paste.Id + "\n"))
}
