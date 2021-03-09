// Package test will hold the integration test cases
package main

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
var ss i.IvmSS
var sid string

// Testing create a New Session
func TestNewSession(t *testing.T) {

	var err error

	// Testing SESSION [Session Repository]

	gsm, err = i.NewSesman(i.Memory, cfg)
	if err != nil {
		t.Errorf("Unexpected error %#v", err.Error())
	}

	t.Run("Start a NEW (no cookie) session",
		func(t *testing.T) {

			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)

			rr := httptest.NewRecorder()

			ss = gsm.SessionStart(rr, req)

			// Save the session id for the next request cycle
			sid = ss.SessionID()
			fmt.Printf("session ID: %#v\n", ss.SessionID())

			hc := rr.Header().Get("Set-Cookie")
			if hc == "" {
				t.Errorf("Unable to set cookie to the response")
			}
		})

	t.Run("Allocate session from the request's cookie sid",
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
		})

	t.Run("...xxx...",
		func(t *testing.T) {

		})
}

// Test Exists
func TestExists(t *testing.T) {
	t.Run("Test Exists with missing sid",
		func(t *testing.T) {
			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			rc := http.Cookie{Name: "ivmid", Value: ""}
			req.AddCookie(&rc)

			rr := httptest.NewRecorder()
			if ok, err := gsm.Exists(rr, req); !ok && err != nil {
				if err != i.ErrUnknownSessionID {
					t.Errorf("Unexpected error: %#v\n", err.Error())
				}
			}

		})

	t.Run("Expect a session to exists",
		func(t *testing.T) {
			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			rc := http.Cookie{Name: "ivmid", Value: sid}
			req.AddCookie(&rc)

			rr := httptest.NewRecorder()
			if ok, err := gsm.Exists(rr, req); !ok || err != nil {
				t.Errorf("Unexpected error %#v or OK value is %#v\n", err.Error(), ok)
			}
		})

	t.Run("Expect a session to NOT exists",
		func(t *testing.T) {
			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			rc := http.Cookie{Name: "ivmid", Value: "...xxx..."}
			req.AddCookie(&rc)

			rr := httptest.NewRecorder()
			if ok, err := gsm.Exists(rr, req); ok || err != nil {
				t.Errorf("Unexpected error %#v or positive OK value %#v\n", err.Error(), ok)
			}
		})
}

// Test Destroy a session
func TestDestroy(t *testing.T) {

	t.Run("Destroy a session",
		func(t *testing.T) {

			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			rc := http.Cookie{Name: "ivmid", Value: sid}
			req.AddCookie(&rc)

			rr := httptest.NewRecorder()
			gsm.Destroy(rr, req)

		})
}
