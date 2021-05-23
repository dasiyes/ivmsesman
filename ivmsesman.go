// Package ivmsesman provides fetures for sessions management.
package ivmsesman

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
)

//
// TODO: Review the session manager design to match the guidlines from (https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
// The stored information can include the client IP address, User-Agent, e-mail, username, user ID, role, privilege level, access rights, language preferences, account ID, current state, last login, session timeouts, and other internal session details.
// If the session objects and properties contain sensitive information, such as credit card numbers, it is required to duly encrypt and protect the session management repository.

// Sesman is the session manager object to be used for managing sessions
type Sesman struct {
	sessions SessionRepository
	lock     sync.Mutex
	cfg      *SesCfg
}

// SesCfg configures the session that will be created
type SesCfg struct {
	CookieName      string
	Maxlifetime     int64
	VisitCookieName string
	ProjectID       string
}

type ssProvider int

const (
	// Memory - defines the session store in the memory
	Memory ssProvider = 10 * iota

	// Firestore - defines the session store in the GCp Firestore native mode
	Firestore

	// Redis - defines the session store in a Redis instance
	Redis
)

// Converts the ssProvider int value to a string
func (ssp ssProvider) String() string {
	switch ssp {
	case Memory:
		return "Memory"
	case Firestore:
		return "FireStore"
	case Redis:
		return "Redis"
	default:
		return ""
	}
}

var providers = make(map[string]SessionRepository)

// Custom key for session obj in the request context
type SessionCtxKey int

var sck SessionCtxKey = 0

// NewSesman will create a new Session Manager
func NewSesman(ssProvider ssProvider, cfg *SesCfg) (*Sesman, error) {
	provider, ok := providers[ssProvider.String()]
	if !ok {
		return nil, fmt.Errorf("Sesman: unknown session store type %q ", ssProvider.String())
	}
	if cfg == nil || cfg.CookieName == "" {
		return nil, fmt.Errorf("Sesman: Missing or invalid Session Manager Configuration")
	}
	return &Sesman{sessions: provider, cfg: cfg}, nil
}

// SessionRepository interface for the session storage
type SessionRepository interface {
	// NewSession will initiate a new session and return its object
	NewSession(sid string) (SessionStore, error)

	// FindOrCreate will search the repository for a session id and if not found will create a new one with the given id
	FindOrCreate(sid string) (SessionStore, error)

	//Exists will check the session storage for a session id
	Exists(sid string) bool

	// FindAll will return slice of all active sessions
	// TODO: FindAll could be expensive. Think if there is a real use-case about it
	// FindAll() []*SessionStore

	//ActiveSessions will return the number of the active sessions in the session store
	ActiveSessions() int

	// Destroy will delete a session from the repository
	Destroy(sid string) error

	// SessionGC will clean the expired sessions
	SessionGC(maxLifeTime int64)

	// UpdateTimeAccessed will refresh the time when the session has been last time accessed
	UpdateTimeAccessed(sid string) error

	// Flush will delete all data
	Flush() error
}

// SessionStore is session store implemenation of interfce to the valid opertions over a session
type SessionStore interface {

	// Set a session key-value
	Set(key, value interface{}) error

	// Get the session value by its key
	Get(key interface{}) interface{}

	// Delete the session by its key
	Delete(key interface{}) error

	// SessionID returns the current session id
	SessionID() string

	// GetLTA will return the LastTimeAccessedAt
	GetLTA() time.Time

	// TODO: Find a use-case to implement this method
	// SessionRelease will release the resource, save the data to presistance storage and return the data to the request
	// SessionRelease(w http.ResponseWriter)
}

// RegisterProvider registers a new provider of session storage for the session management.
func RegisterProvider(name ssProvider, provider SessionRepository) {

	if _, dup := providers[name.String()]; dup {
		panic("SesMan: Provider " + name.String() + " is already registered")
	}

	if provider == nil {
		panic("SesMan: Register function needs not-null provider")
	}

	providers[name.String()] = provider
}

// sessionID will create a new, unique sessin ID. [more here](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html#session-id-properties)
func (sm *Sesman) sessionID() string {
	var sid ksuid.KSUID
	defer func() string {
		if r := recover(); r != nil {
			return ""
		}
		return sid.String()
	}()
	sid = ksuid.New()
	return sid.String()
}

// SessionStart allocate (existing session id) or create a new session if it does not exists for validating user oprations
func (sm *Sesman) SessionStart(w http.ResponseWriter, r *http.Request) (SessionStore, error) {

	sm.lock.Lock()
	defer sm.lock.Unlock()

	var session SessionStore

	fmt.Printf("[SessionStart] cookie name: %v, cookie header: %v\n", sm.cfg.CookieName, r.Header.Get("Cookie"))
	cookie, err := r.Cookie(sm.cfg.CookieName)
	if err != nil && err == http.ErrNoCookie {

		fmt.Printf("while getting cookie error: %v\n", err)

		sid := sm.sessionID()
		session, err = sm.sessions.NewSession(sid)
		if err != nil {
			return nil, err
		}

		cookie := http.Cookie{
			Name:     sm.cfg.CookieName,
			Value:    url.QueryEscape(sid),
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   int(sm.cfg.Maxlifetime)}

		http.SetCookie(w, &cookie)

	} else {

		sid, err := url.QueryUnescape(cookie.Value)
		if err != nil {
			return nil, fmt.Errorf("unable to unescape the session id, error %v", err)
		}
		fmt.Printf("cookie/session id value: %v\n", sid)
		session, err = sm.sessions.FindOrCreate(sid)
		if err != nil {
			return nil, fmt.Errorf("unable to acquire the session id %v , error %v", sid, err)
		}
	}

	return session, nil
}

// Manager - Middleware to work with Session manager
func (sm *Sesman) Manager(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Enhancing security
		w.Header().Set("X-XSS-Protection", "1;mode=block")
		w.Header().Set("X-Frame-Options", "deny")

		session, err := sm.SessionStart(w, r)
		if err != nil {
			fmt.Printf("dropping the request due to session management error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		sid := session.Get("Sid").(string)
		sesValue := session.Get("Value").(map[interface{}]interface{})
		sesState := sesValue["state"].(string)

		fmt.Printf("... acquired state is: %v, sessionID: %v\n", sesState, sid)

		ctx := context.WithValue(r.Context(), sck, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ActiveSessions will return the number of the active sessions in the session store
func (sm *Sesman) ActiveSessions() int {
	return sm.sessions.ActiveSessions()
}

// GetLastAccessedAt will return the seconds since Epoch when the session was lastly accessed.
// TODO: Consider if there will be use-case for this to be implemented...
// func (sm *Sesman) GetLastAccessedAt() int64 {
// 	return 0
// }

// Destroy sessionid
func (sm *Sesman) Destroy(w http.ResponseWriter, r *http.Request) {

	cookie, err := r.Cookie(sm.cfg.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	sm.sessions.Destroy(cookie.Value)
	expiration := time.Now()

	cookie = &http.Cookie{
		Name:     sm.cfg.CookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		Expires:  expiration,
		MaxAge:   -1,
	}

	http.SetCookie(w, cookie)
}

// GC is a global clean for expired sessions. It needs to be started in the calling func
//
// Example:
//	func init() {
//		go globalSessions.GC()
//	}
// The GC makes full use of the timer function in the time package. It automatically calls GC when the session times out, ensuring that all sessions are usable during maxLifeTime.
// TODO: A similar solution can be used to count online users.
func (sm *Sesman) GC() {

	sm.lock.Lock()
	defer sm.lock.Unlock()

	sm.sessions.SessionGC(sm.cfg.Maxlifetime)
	time.AfterFunc(time.Duration(sm.cfg.Maxlifetime), func() { sm.GC() })
}

// Exists will check the session repository for a session by its id and return the result as bool
func (sm *Sesman) Exists(w http.ResponseWriter, r *http.Request) (bool, error) {

	cookie, err := r.Cookie(sm.cfg.CookieName)
	if err != nil || cookie.Value == "" {
		return false, ErrUnknownSessionID
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	return sm.sessions.Exists(cookie.Value), nil
}

// ErrUnknownSessionID  will be returned when a session id is required for a operation but it is missing or wrong value
var ErrUnknownSessionID = errors.New("unknown session id")
