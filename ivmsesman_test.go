package ivmsesman

import (
	"strings"
	"testing"
)

// Creates new Session Configuration
var cfg *SesCfg = &SesCfg{
	CookieName:  "ivmid",
	Maxlifetime: 3600,
}

var gsm *Sesman

var prvds map[string]SessionRepository

// Testing the initiation of the Session Store Providers
func TestInitiateProviders(t *testing.T) {
	mp := initiateProviders()
	if _, ok := mp["Memory"]; !ok {
		t.Errorf("Memory is expected to be an accepted SS provider")
	}
	if _, ok := mp["FireStore"]; !ok {
		t.Errorf("FireStore is expected to be an accepted SS provider")
	}
	if _, ok := mp["Redis"]; !ok {
		t.Errorf("Redis is expected to be an accepted SS provider")
	}
	if _, ok := mp["SomethingElse"]; ok {
		t.Errorf("SomethingElse is NOT expected to be an accepted SS provider")
	}
}

// Testing Register provider
func TestRegisterProvider(t *testing.T) {

	// Initiate memory provider for the following test cases
	providers[Memory.String()] = nil

	t.Run("Register DUPLICATED provider",
		func(t *testing.T) {
			defer func() {
				if r2 := recover(); r2 != nil {
					rm := r2.(string)
					if rm != "SesMan: Provider "+Memory.String()+" is already registered" {
						t.Errorf("Unexpected error: %#v\n", r2)
					}
				}
			}()

			RegisterProvider(Memory, nil)
		})

	t.Run("Register NIL provider",
		func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if r.(string) != "SesMan: Register function needs not-null provider" {
						t.Errorf("Unexpected error: %#v\n", r)
					}
				}
			}()

			RegisterProvider(Firestore, nil)
		})
}

// Testing the creation of new Session Manager object
func TestNewSessman(t *testing.T) {
	t.Run("Uknown provider type",
		func(t *testing.T) {

			_, err := NewSesman(42, cfg)
			if err == nil {
				t.Errorf("Failed to capture expected error!")
			}
			if !strings.HasPrefix(err.Error(), "Sesman: unknown session store type") {
				t.Errorf("Unexpected error: %#v", err.Error())
			}
		})

	t.Run("Empty configuration object",
		func(t *testing.T) {

			_, err := NewSesman(Memory, &SesCfg{})
			if err == nil {
				t.Errorf("Failed to capture expected error!")
			}
			if err.Error() != "Sesman: Missing or invalid Session Manager Configuration" {
				t.Errorf("Unexpected error: %#v", err.Error())
			}

			_, err2 := NewSesman(Memory, &SesCfg{CookieName: ""})
			if err2 == nil {
				t.Errorf("Failed to capture expected error!")
			}
			if err2.Error() != "Sesman: Missing or invalid Session Manager Configuration" {
				t.Errorf("Unexpected error: %#v", err2.Error())
			}
		})

	t.Run("Valid provider type",
		func(t *testing.T) {
			gsm, err := NewSesman(Memory, cfg)
			if err != nil {
				t.Errorf("Wanted no error, got: %q", err.Error())
			}

			cn := gsm.cfg.CookieName
			if cn != "ivmid" {
				t.Errorf("Expected cookie name 'ivmid', got: %#v", cn)
			}

			mlt := gsm.cfg.Maxlifetime
			if mlt != 3600 {
				t.Errorf("Expected maxLifeTime value 3600, got: %#v", mlt)
			}
		})
}

func TestSessionID(t *testing.T) {

	var err error
	var sid string

	gsm, err = NewSesman(Memory, cfg)
	if err != nil {
		t.Errorf("Unexpected error %#v", err.Error())
	}

	t.Run("Session ID generate",
		func(t *testing.T) {
			sid = gsm.sessionID()
			if sid == "" {
				t.Errorf("Session ID generation returned empty string as ID")
			}
			// fmt.Printf("generated sid: %#v\n", sid)
			if len(sid) < 27 {
				t.Errorf("Session ID length is below expected 27 chars")
			}
		})
}
