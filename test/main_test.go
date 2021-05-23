// Package test will hold the integration test cases
package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
var ss i.SessionStore
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

			ss, _ = gsm.SessionStart(rr, req)

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
			ss, err = gsm.SessionStart(rr, req)

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

			var as int = gsm.ActiveSessions()
			fmt.Printf("active sessions: %d\n", as)
			if as != 1 {
				t.Errorf("Unexpected active sessions. Wanted 1, but got %d\n", as)
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

// Test UpdateTimeAccessed will test the session's property UdateTimeAccess
func TestUpdateTimeAccessed(t *testing.T) {
	// Both session store's methods NewSession and FindOrCreate as well as session's GET method will update the property.
	ltaa := ss.GetLTA()
	fmt.Printf("Last time accessed at: %d, vs Now: %d\n", ltaa.UnixNano(), time.Now().UnixNano())
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

// Test ActiveSessions - expect to be 0 (after destroy)
func TestActiveSessions(t *testing.T) {
	var as int = gsm.ActiveSessions()
	fmt.Printf("active sessions: %d", as)
	if as != 0 {
		t.Errorf("Unexpected active sessions. Wanted 0, but got %d\n", as)
	}
}

// TODO: Write test for the Firestore session store
