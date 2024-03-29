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

	"github.com/go-chi/chi/v5/middleware"
	"github.com/segmentio/ksuid"
)

// Key to use when setting the request ID.
type ctxKeySessionObj int

// RequestIDKey is the key that holds the unique request ID in a request context.
const SessionObjKey ctxKeySessionObj = 0

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
	BLCleanInterval int64
}

type ssProvider int

const (
	// Memory - defines the session store in the memory
	Memory ssProvider = 10 * iota

	// Firestore - defines the session store in the GCP Firestore native mode
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
//type SessionCtxKey string
//var sckState SessionCtxKey = "sessionState"

// NewSesman will create a new Session Manager
func NewSesman(ssProvider ssProvider, cfg *SesCfg) (*Sesman, error) {
	provider, ok := providers[ssProvider.String()]
	if !ok {
		return nil, fmt.Errorf("Sesman: unknown session store type %q ", ssProvider.String())
	}
	if cfg == nil || cfg.CookieName == "" || cfg.ProjectID == "" {
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
	DestroySID(sid string) error

	// SessionGC will clean the expired sessions
	SessionGC(maxLifeTime int64)

	// UpdateTimeAccessed will refresh the time when the session has been last time accessed
	UpdateTimeAccessed(sid string) error

	// UpdateSessionState will update the state value with one provided
	UpdateSessionState(sid string, state string) error

	// UpdateCodeVerifier will update the code verifier (cove) value assigned to the session id
	UpdateCodeVerifier(sid, cove string) error

	// SaveCodeChallengeAndMethod - at step2 of AuthorizationCode flow
	SaveCodeChallengeAndMethod(sid, coch, mth, code, ru string) error

	// Flush will delete all data
	Flush() error

	// GetSessionAuthCode will return the authorization code for a session, if it is InAuth and the code did not expire.
	GetAuthCode(sid string) map[string]string

	// UpdateAuthSession - update state, access and refresh tokens values for auth session
	UpdateAuthSession(sid, at, rt, uid string) error

	//Blacklisting
	Blacklisting(ip, path string, data interface{})

	// IsIPExistInBL will check the black list
	IsIPExistInBL(ip string) bool

	// BLClean is a support function to clean the Blacklist on regular base
	BLClean()
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

// SessionManager allocate (existing session id) or create a new session if it does not exists for validating user oprations
func (sm *Sesman) SessionManager(w http.ResponseWriter, r *http.Request) (SessionStore, error) {

	sm.lock.Lock()
	defer sm.lock.Unlock()

	var session SessionStore

	// [ ]: remove after debug
	fmt.Printf("searching for cookie name: [%s]\n", sm.cfg.CookieName)
	cookie, err := r.Cookie(sm.cfg.CookieName)

	if err == http.ErrNoCookie {

		sid := sm.sessionID()

		// TODO: remove after debug
		fmt.Printf("[SessionManager-1] generated sid: %v\n", sid)

		session, err = sm.sessions.NewSession(sid)
		if err != nil {
			return nil, fmt.Errorf("error creating a new session: %v", err)
		}

		// TODO: remove after debug
		fmt.Printf("[SessionManager-2] session ID: %v\n", session.SessionID())

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

		session, err = sm.sessions.FindOrCreate(sid)
		if err != nil {
			return nil, fmt.Errorf("unable to acquire the session id %v , error %v", sid, err)
		}
	}

	return session, nil
}

// MWManager - is a Middleware Handler that proxy the Session Manager
func (sm *Sesman) MWManager(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Enhancing security
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "deny")

		session, err := sm.SessionManager(w, r)
		if err != nil || session == nil {
			w.Header().Set("Connection", "close")
			fmt.Printf("[Error] dropping the request due to session management error: %v\n", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		sesStateValue := session.Get("state").(string)
		r.Header.Set("X-Session-State", sesStateValue)

		ctx := r.Context()
		rid := middleware.GetReqID(ctx)

		ctx = context.WithValue(ctx, SessionObjKey, session)

		// TODO: remove after debug
		sid := session.SessionID()
		fmt.Printf("[mw MWManager] request id [%s] session id [%v], with session state [%v] found in the request\n", rid, sid, sesStateValue)

		if sesStateValue != "Authed" {
			// Delete previously set ia cookie
			w.Header().Add("Set-Cookie", "ia=deleted; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT")
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ActiveSessions will return the number of the active sessions in the session store
func (sm *Sesman) ActiveSessions() int {
	return sm.sessions.ActiveSessions()
}

// UpdateCodeVerifier will update the code verifier (cove) value assigned to the session id
func (sm *Sesman) UpdateCodeVerifier(sid, cove string) error {
	return sm.sessions.UpdateCodeVerifier(sid, cove)
}

// SaveACA - at step2 of AuthorizationCode flow save Athorization Code Attributes
func (sm *Sesman) SaveACA(sid, coch, mth, code, ru string) error {
	return sm.sessions.SaveCodeChallengeAndMethod(sid, coch, mth, code, ru)
}

// GetSessionAuthCode will return the authorization code for a session, if it is InAuth
func (sm *Sesman) GetAuthCode(sid string) map[string]string {
	return sm.sessions.GetAuthCode(sid)
}

// Blacklisting the ip from the func argument
func (sm *Sesman) AddBlacklisting(ip, path string, data interface{}) {
	sm.sessions.Blacklisting(ip, path, data)
}

// IsBlackListed - checks if the ip is blacklisted
func (sm *Sesman) IsBlackListed(ip string) bool {
	return sm.sessions.IsIPExistInBL(ip)
}

// GetAuthSessAT - will extract the value of the attribute sent in the func
func (sm *Sesman) GetAuthSessAT(ctx context.Context, val_att string) string {
	if ctx == nil {
		fmt.Printf("\n...nil context!\n")
		return ""
	}
	if sess, ok := ctx.Value(SessionObjKey).(SessionStore); ok {
		var at = sess.Get(val_att).(string)
		return at
	}
	return ""
}

// GetAuthSessionAttribute - will use the request to get the session id (either context or the session cookie) and return the requested session's attribute
func (sm *Sesman) GetAuthSessionAttribute(r *http.Request, att_name string) (atrb interface{}, err error) {

	cookie, err := r.Cookie(sm.cfg.CookieName)
	if err != nil || cookie.Value == "" {
		return nil, ErrUnknownSessionID
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	if !sm.sessions.Exists(cookie.Value) {
		return nil, ErrInvalidSessionID
	}

	ses, errs := sm.sessions.FindOrCreate(cookie.Value)
	if errs != nil {
		return nil, fmt.Errorf("unable to find session id %s, error: %v", cookie.Value, errs)
	}

	return ses.Get(att_name), nil
}

// GetLastAccessedAt will return the seconds since Epoch when the session was lastly accessed.
func (sm *Sesman) GetLastAccessedAt() int64 {
	// [ ] implement getting the LAT
	return 0
}

// Destroy sessionid
func (sm *Sesman) Destroy(w http.ResponseWriter, r *http.Request) {

	cookie, err := r.Cookie(sm.cfg.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	_ = sm.sessions.DestroySID(cookie.Value)
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
//
//	func init() {
//		go globalSessions.GC()
//	}
//
// The GC makes full use of the timer function in the time package. It automatically calls GC when the session times out, ensuring that all sessions are usable during maxLifeTime.
// TODO: A similar solution can be used to count online users.
func (sm *Sesman) GC() {

	sm.lock.Lock()
	defer sm.lock.Unlock()

	// TODO: find a way to prevent app crashing with panic
	sm.sessions.SessionGC(sm.cfg.Maxlifetime)
	time.AfterFunc(time.Duration(sm.cfg.Maxlifetime), func() { sm.GC() })
}

// BLC is a support function to clean the blacklist collection in FireStore on regular RunServer
func (sm *Sesman) BLC() {

	sm.sessions.BLClean()
	intv := time.Duration(sm.cfg.BLCleanInterval) * time.Second
	time.AfterFunc(intv, func() {
		fmt.Printf("started at: %v, with interval: %v\n", time.Now(), intv)
		sm.BLC()
	})
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

// Change state will be using the custom request header X-Session-State to handle the state defined by other services like API gateway and auth-service
func (sm *Sesman) ChangeState(w http.ResponseWriter, r *http.Request) (bool, error) {

	cookie, err := r.Cookie(sm.cfg.CookieName)
	if err != nil || cookie.Value == "" {
		return false, ErrUnknownSessionID
	}

	var stateVal = r.Header.Get("X-Session-State")
	if stateVal == "" {
		return false, fmt.Errorf("missing not empty value for the new state in the request custome header x-session-state")
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	err = sm.sessions.UpdateSessionState(cookie.Value, stateVal)
	if err != nil {
		return false, err
	}

	fmt.Printf("Session id %v state MSUT be changed to %v\n", cookie.Value, stateVal)
	return true, nil
}

// SessionAuth changes an existing session in state "InAuth" to a new id and state "Authed"
func (sm *Sesman) SessionAuth(w http.ResponseWriter, r *http.Request, at, rt, uid string) error {

	cookie, err := r.Cookie(sm.cfg.CookieName)
	if err != nil || cookie.Value == "" {
		return ErrUnknownSessionID
	}

	sm.lock.Lock()
	defer sm.lock.Unlock()

	if !sm.sessions.Exists(cookie.Value) {
		return ErrInvalidSessionID
	}

	err = sm.sessions.DestroySID(cookie.Value)
	if err != nil {
		return fmt.Errorf("error distroying the old `InAuth` session: %s", err.Error())
	}

	nsid := sm.sessionID()
	_, err = sm.sessions.NewSession(nsid)
	if err != nil {
		return fmt.Errorf("error creating Authed session: %s", err.Error())
	}

	err = sm.sessions.UpdateAuthSession(nsid, at, rt, uid)
	if err != nil {
		return fmt.Errorf("error updating Authed session: %s", err.Error())
	}

	nsCookie := http.Cookie{
		Name:     sm.cfg.CookieName,
		Value:    url.QueryEscape(nsid),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sm.cfg.Maxlifetime)}

	http.SetCookie(w, &nsCookie)

	return nil
}

// ErrUnknownSessionID  will be returned when a session id is required for a operation but it is missing or wrong value
var ErrUnknownSessionID = errors.New("unknown session id")

// ErrInvalidSessionID  will be returned when a session id is required for a operation but it does not exists.
var ErrInvalidSessionID = errors.New("invalid session id")
