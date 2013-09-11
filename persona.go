package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/sessions"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

type PersonaResponse struct {
	Status   string `json: "status"`
	Email    string `json: "email"`
	Audience string `json: "audience"`
	Expires  int64  `json: "expires"`
	Issuer   string `json: "issuer"`
	Reason   string `json: "reason,omitempty"`
}

type PersonaHandler interface {
	IsLoggedIn(r *http.Request) bool
	GetLoginName(r *http.Request) string
	Login(verifierURL string, w http.ResponseWriter, r *http.Request) error
	Logout(w http.ResponseWriter, r *http.Request)
}

type Persona struct {
	hostname string
	store    *sessions.CookieStore
}

func NewPersonaHandler(hostname, cookie string) PersonaHandler {
	store := sessions.NewCookieStore([]byte(cookie))
	return Persona{hostname, store}
}

func (self Persona) IsLoggedIn(r *http.Request) bool {
	session, _ := self.store.Get(r, "persona-session")
	email := session.Values["email"]
	return email != nil
}

func (self Persona) GetLoginName(r *http.Request) string {
	session, _ := self.store.Get(r, "persona-session")
	email := session.Values["email"]

	if str, ok := email.(string); ok {
		return str
	}
	return ""
}

func (self Persona) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := self.store.Get(r, "persona-session")
	session.Values["email"] = nil
	session.Save(r, w)
}

func (self Persona) Login(verifierURL string, w http.ResponseWriter, r *http.Request) error {
	assertion := r.FormValue("assertion")
	if assertion == "" {
		return fmt.Errorf("Assertion not provided")
	}

	form := url.Values{
		"assertion": {assertion},
		"audience": {"http://" + self.hostname }}

	vr, err := http.PostForm(verifierURL, form)
	if err != nil {
		return fmt.Errorf("Failed to verify: " + err.Error())
	}

	body, err := ioutil.ReadAll(vr.Body)
	if err != nil {
		return fmt.Errorf("Failed to read verifier's response")
	}

	pr := &PersonaResponse{}
	if err = json.Unmarshal(body, pr); err != nil {
		return fmt.Errorf("Failed to unmarshal verifier's response")
	}

	if pr.Status != "okay" {
		log.Println("Persona failed to verify: " + pr.Reason)
		return fmt.Errorf("Persona failed to verify")
	}

	session, _ := self.store.Get(r, "persona-session")
	session.Values["email"] = pr.Email
	session.Save(r, w)

	return nil
}
