// Package test will hold the integration test cases
package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	i "github.com/dasiyes/ivmsesman"
	_ "github.com/dasiyes/ivmsesman/providers/firestore"
	_ "github.com/dasiyes/ivmsesman/providers/inmem"
)

// func TestMain(m *testing.M) {
// 	// command to start firestore emulator
// 	cmd := exec.Command("gcloud", "beta", "emulators", "firestore", "start", "--host-port=localhost")

// 	// this makes it killable
// 	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

// 	// we need to capture it's output to know when it's started
// 	stderr, err := cmd.StderrPipe()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer stderr.Close()

// 	// start her up!
// 	if err := cmd.Start(); err != nil {
// 		log.Fatal(err)
// 	}

// 	// ensure the process is killed when we're finished, even if an error occurs
// 	// (thanks to Brian Moran for suggestion)
// 	var result int
// 	defer func() {
// 		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
// 		os.Exit(result)
// 	}()

// 	// we're going to wait until it's running to start
// 	var wg sync.WaitGroup
// 	wg.Add(1)

// 	// by starting a separate go routine
// 	go func() {
// 		// reading it's output
// 		buf := make([]byte, 256)
// 		for {
// 			n, err := stderr.Read(buf[:])
// 			if err != nil {
// 				// until it ends
// 				if err == io.EOF {
// 					break
// 				}
// 				log.Fatalf("reading stderr %v", err)
// 			}

// 			if n > 0 {
// 				d := string(buf[:n])

// 				// only required if we want to see the emulator output
// 				log.Printf("%s", d)

// 				// checking for the message that it's started
// 				if strings.Contains(d, "Dev App Server is now running") {
// 					wg.Done()
// 				}

// 				// and capturing the FIRESTORE_EMULATOR_HOST value to set
// 				pos := strings.Index(d, FirestoreEmulatorHost+"=")
// 				if pos > 0 {
// 					host := d[pos+len(FirestoreEmulatorHost)+1 : len(d)-1]
// 					os.Setenv(FirestoreEmulatorHost, host)
// 				}
// 			}
// 		}
// 	}()

// 	// wait until the running message has been received
// 	wg.Wait()

// 	// now it's running, we can run our unit tests
// 	result = m.Run()
// }

// const FirestoreEmulatorHost = "FIRESTORE_EMULATOR_HOST"

// ##################################### my real testing from here on ################

// Creates new Session Configuration
var cfg *i.SesCfg = &i.SesCfg{
	CookieName:  "ivmid",
	Maxlifetime: 3600,
	ProjectID:   "test",
}

// ######### Testing Session Start && FindOrCreate #########

var gsm *i.Sesman
var ss i.SessionStore
var sid string

// Testing create a New Session
func TestNewSession(t *testing.T) {

	var err error

	// Testing SESSION [Session Repository]
	gsm, err = i.NewSesman(i.Memory, cfg)
	if err != nil {
		t.Errorf("[Memory] Unexpected error %#v", err.Error())
	}

	t.Run("[Memory] Test SessionStart [no cookie]",
		func(t *testing.T) {

			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			ss, _ = gsm.SessionStart(rr, req)
			if ss == nil {
				t.Errorf("error while SessionStart %v\n", ss)
			}

			// Save the session id for the next request cycle
			sid = ss.SessionID()
			if sid == "" {
				t.Errorf("empty value for session id from NewSession %v\n", sid)
			}
			state := ss.Get("state").(string)
			if state == "" {
				t.Errorf("empty value for state from NewSession %v\n", state)
			}
			if strings.ToLower(state) != "new" {
				t.Errorf("Expected value `new` - actual value %v\n", state)
			}
		})

	t.Run("[Memory] Test FindOrCreate [existing but empty cookie `ivmid`]",
		func(t *testing.T) {

			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			rc := http.Cookie{Name: cfg.CookieName, Value: ""}
			req.AddCookie(&rc)

			rr := httptest.NewRecorder()
			ss, err = gsm.SessionStart(rr, req)
			if ss == nil {
				t.Errorf("error while SessionStart %v\n", ss)
			}

			// Save the session id for the next request cycle
			sid = ss.SessionID()
			if sid == "" {
				t.Errorf("empty value for session id from FindOrCreate %v\n", sid)
			}
			state := ss.Get("state").(string)
			if state == "" {
				t.Errorf("empty value for state from FindOrCreate %v\n", state)
			}
			if strings.ToLower(state) != "new" {
				t.Errorf("Expected value `new` - actual value %v\n", state)
			}

		})

	t.Run("[Memory] Test FindOrCreate return existing session [existing cookie]",
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

			state := ss.Get("state").(string)
			if state == "" {
				t.Errorf("empty value for state from FindOrCreate %v\n", state)
			}
			if strings.ToLower(state) != "new" {
				t.Errorf("Expected value `new` - actual value %v\n", state)
			}
		})

	t.Run("[Memory] ...xxx...",
		func(t *testing.T) {

		})
}

// Test Exists
func TestExists(t *testing.T) {
	t.Run("[Memory] Test Exists with missing sid",
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

	t.Run("[Memory] Expect a session to exists",
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
			if as != 2 {
				t.Errorf("Unexpected active sessions. Wanted 1, but got %d\n", as)
			}
		})

	t.Run("[Memory] Expect a session to NOT exists",
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

	t.Run("[Memory] Destroy a session",
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
	fmt.Printf("active sessions: %d\n", as)
	if as != 1 {
		t.Errorf("Unexpected active sessions. Wanted 0, but got %d\n", as)
	}
}

// ############# Testing Firestore Provider ###############
// Testing create a New Session - firestore
func TestFirestoreNewSession(t *testing.T) {

	var err error
	sid = ""

	gsm, err = i.NewSesman(i.Firestore, cfg)
	if err != nil {
		t.Errorf("Unexpected error %#v", err.Error())
	}

	t.Run("[Firestore] Test SessionStart [no cookie]",
		func(t *testing.T) {

			// simulate a http request
			req, _ := http.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			ss, err = gsm.SessionStart(rr, req)
			if err != nil {
				t.Errorf("error while starting new session %v", err)
			}
			fmt.Printf("sessionstore id: %v\n", ss.SessionID())

			// Save the session id for the next request cycle
			sid = ss.SessionID()
			if sid == "" {
				t.Errorf("empty value for session id from NewSession %v\n", sid)
			}
			state := ss.Get("state").(string)
			if state == "" {
				t.Errorf("empty value for state from NewSession %v\n", state)
			}
			if strings.ToLower(state) != "new" {
				t.Errorf("Expected value `new` - actual value %v\n", state)
			}
		})

	t.Run("[Firestore] Allocate session from the request's cookie sid",
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

	t.Run("[Firestore] ...xxx...",
		func(t *testing.T) {

		})
}
