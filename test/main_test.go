package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	i "github.com/dasiyes/ivmsesman"
	_ "github.com/dasiyes/ivmsesman/providers/inmem"
)

// Creates new Session Configuration
var cfg *i.SesCfg = &i.SesCfg{
	CookieName:  "ivmid",
	Maxlifetime: 3600,
}

// ######### Testing Session Manager #########

var gsm *i.Sesman

// Testing create a New Session
func TestNewSession(t *testing.T) {

	var err error
	var sid string
	var ss i.IvmSS

	// Testing SESSION [Session Repository]

	gsm, err = i.NewSesman(i.Memory, cfg)
	if err != nil {
		t.Errorf("Unexpected error %#v", err.Error())
	}

	t.Run("Start a NEW (no cookie) session",
		func(t *testing.T) {

			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			// rc := http.Cookie{Name: "ivmid", Value: sid}
			// req.AddCookie(&rc)

			rr := httptest.NewRecorder()

			ss = gsm.SessionStart(rr, req)

			// Save the session id for the next request cycle
			sid = ss.SessionID()
			fmt.Printf("session ID: %#v\n", ss.SessionID())

			hc := rr.Header().Get("Set-Cookie")
			// TODO: expand the testing parameters here
			fmt.Printf("Cookies: %#v\n", hc)
		})

	t.Run("Allocate session from cookie sid",
		func(t *testing.T) {

			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			rc := http.Cookie{Name: "ivmid", Value: sid}
			req.AddCookie(&rc)

			rr := httptest.NewRecorder()
			ss = gsm.SessionStart(rr, req)

			if sid != ss.SessionID() {
				t.Errorf("Unexpected difference for session id\n expected: %#v,\n got: %#v", sid, ss.SessionID())
			}

			rr.Header().Set("Cookie", "ivmid="+sid)

			hc := rr.Result().Header.Get("Cookie")
			fmt.Printf("Cookies: %#v\n", hc)

			// TODO: expand the testing parameters here

		})

	t.Run("Create new Session object",
		func(t *testing.T) {

		})

	t.Run("Empty configuration object",
		func(t *testing.T) {

		})
}
