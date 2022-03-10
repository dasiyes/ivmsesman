package main

// the _ will invoke the init func at import time
import (
	"fmt"
	"net/http"
	"os"
	"text/template"

	"github.com/dasiyes/ivmsesman"
	_ "github.com/dasiyes/ivmsesman/providers/inmem"
)

var globalSesMan *ivmsesman.Sesman

func main() {

	var err error
	var cfg *ivmsesman.SesCfg = &ivmsesman.SesCfg{
		CookieName:  "ivmid",
		Maxlifetime: 3600,
	}

	// Create a new Session Manager
	globalSesMan, err = ivmsesman.NewSesman(ivmsesman.Memory, cfg)
	if err != nil {
		fmt.Printf("Unable to initiate valid session manager %q", err)
		os.Exit(1)
	}

	// Running sessions GC in a separate routine
	go globalSesMan.GC()

}

// Here is an example that uses sessions for a login operation.
func login(w http.ResponseWriter, r *http.Request) {
	sess, _ := globalSesMan.SessionManager(w, r)

	r.ParseForm()
	if r.Method == "GET" {
		t, _ := template.ParseFiles("login.gtpl")
		w.Header().Set("Content-Type", "text/html")
		t.Execute(w, sess.Get("username"))
	} else {
		sess.Set("username", r.Form["username"])
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
