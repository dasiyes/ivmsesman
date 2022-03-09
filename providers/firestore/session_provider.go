package firestoredb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"github.com/dasiyes/ivmsesman"
)

// TODO [dev]: move the value of `sessions` and `blacklist` in the configuration
var pder = &SessionProvider{collection: "sessions", blacklist: "blacklist"}

// SessionProvider is the DAL holding the methods for database operations fr the SessionManager
type SessionProvider struct {
	client     *firestore.Client
	collection string
	// the name of the collection for the blacklist
	blacklist string
}

// FindOrCreate will first search the store for a session value with provided sid. If not not found, a new session value will be created and stored in the session store
func (pder *SessionProvider) FindOrCreate(sid string) (ivmsesman.SessionStore, error) {

	var ss Session = Session{}

	docses, err := pder.client.Collection(pder.collection).Doc(sid).Get(context.TODO())
	if err != nil {
		if strings.Contains(err.Error(), "Missing or insufficient permissions") {
			return nil, errors.New("insufficient permissions to read data from the session store")
		} else {
			if docses == nil {
				fmt.Printf("sid: %v was not found in the session store. A new session will be created. Error: %v\n", sid, err)
				return pder.NewSession(sid)
			}
			if strings.Contains(err.Error(), "NotFound") {
				// Recreate the session with the old sid
				nss, err := pder.NewSession(sid)
				if err != nil {
					return nil, fmt.Errorf("err re-create session id: %v, err: %v", sid, err)
				}
				return nss, nil

			} else {
				return nil, fmt.Errorf("err while read session id: %v, err: %v", sid, err)
			}
		}
	}

	err = docses.DataTo(&ss)
	if err != nil {
		return nil, fmt.Errorf("error while converting firstore doc to session object: %v", err)
	}

	return &ss, nil
}

// Destroy will remove a session data from the storage
func (pder *SessionProvider) DestroySID(sid string) error {

	_, err := pder.client.Collection(pder.collection).Doc(sid).Delete(context.TODO())
	if err != nil {
		return err
	}
	return nil
}

// SessionGC cleans all expired sessions
func (pder *SessionProvider) SessionGC(maxlifetime int64) {

	if maxlifetime == 0 {
		maxlifetime = 3600
	}
	iter := pder.client.Collection(pder.collection).Where("TimeAccessed", "<", (time.Now().Unix() - maxlifetime)).Documents(context.TODO())

	var erritr error

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			erritr = fmt.Errorf("err while iterate expired session id: %v, err: %v", doc.Ref.ID, err)
		}
		_, err = doc.Ref.Delete(context.TODO())
		if err != nil {
			erritr = fmt.Errorf("error deleting session id %v, err: %v", doc.Ref.ID, err)
		}
	}
	if erritr != nil {
		fmt.Printf("error stack: %v", erritr)
	}
}

// BLClean - cleaning the Firestore blacklist
func (pder *SessionProvider) BLClean() {
	docs_cnt := 0
	del_docs_cnt := 0

	// set default value for ip caranteen period to 30 days
	// cp := int64(2590000)
	cp := int64(259200) // 3 days

	// treshold value back in the time (default 30 days) after which the blacklisted ip address will be reviewed for cleaning
	to := time.Now().Unix() - cp

	iter := pder.client.Collection(pder.blacklist).Where("created", "<", time.Unix(to, 0)).Documents(context.TODO())
	for {

		d, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("ip address %v, raised an error: %v\n", d.Ref.ID, err)
			continue
		}

		// send the IP address for verification for being good bot
		if nativeReverseDNSLookup(d.Ref.ID) {
			_, err = d.Ref.Delete(context.TODO())
			if err != nil {
				fmt.Printf("while deleting ip %s, an error raised: %v\n", d.Ref.ID, err)
				continue
			}
			del_docs_cnt++
		}
		docs_cnt++
	}
	fmt.Printf(" * blacklist clean summary: %d docs reviewed, %d deleted\n", docs_cnt, del_docs_cnt)
}

// UpdateTimeAccessed will update the time accessed value with now()
func (pder *SessionProvider) UpdateTimeAccessed(sid string) error {
	_, err := pder.client.Collection(pder.collection).Doc(sid).Update(context.TODO(),
		[]firestore.Update{
			{
				Path:  "TimeAccessed",
				Value: time.Now().Unix(),
			},
		})
	if err != nil {
		return fmt.Errorf("err while updating time accessed for sessions id %v, err: %v", sid, err)
	}
	return nil
}

// UpdateSessionState will update the state value with one provided
func (pder *SessionProvider) UpdateSessionState(sid string, state string) error {
	_, err := pder.client.Collection(pder.collection).Doc(sid).Update(context.TODO(),
		[]firestore.Update{
			{
				Path:  "Value.state",
				Value: state,
			},
		})
	if err != nil {
		return fmt.Errorf("err while updating `Value.state` for sessions id %v, err: %v", sid, err)
	}
	return nil
}

// UpdateCodeVerifier will update the code verifier (cove) value assigned to the session id
func (pder *SessionProvider) UpdateCodeVerifier(sid, cove string) error {
	_, err := pder.client.Collection(pder.collection).Doc(sid).Update(context.TODO(),
		[]firestore.Update{
			{
				Path:  "Value.code_verifier",
				Value: cove,
			},
		})
	if err != nil {
		return fmt.Errorf("err while updating `Value.code_verifier` for sessions id %v, err: %v", sid, err)
	}
	return nil
}

// SaveCodeChallengeAndMethod - at step2 of AuthorizationCode flow
func (pder *SessionProvider) SaveCodeChallengeAndMethod(
	sid, coch, mth, code, ru string) error {

	// set code expiration timestamp
	ce := time.Now().Unix() + 60

	_, err := pder.client.Collection(pder.collection).Doc(sid).Update(context.TODO(),
		[]firestore.Update{
			{
				Path:  "Value.code_challenger",
				Value: coch,
			},
			{
				Path:  "Value.code_challenger_method",
				Value: mth,
			},
			{
				Path:  "Value.auth_code",
				Value: code,
			},
			{
				Path:  "Value.code_expire",
				Value: ce,
			},
			{
				Path:  "Value.redirect_uri",
				Value: ru,
			},
			{
				Path:  "Value.state",
				Value: "InAuth",
			},
		})
	if err != nil {
		return fmt.Errorf("err while updating `Value.code_verifier` for sessions id %v, err: %v", sid, err)
	}
	return nil
}

// GetAuthCode will return the authorization code for a session, if it is InAuth
func (pder *SessionProvider) GetAuthCode(sid string) map[string]string {

	now := time.Now().Unix()
	var ac map[string]string = map[string]string{}

	docses, err := pder.client.Collection(pder.collection).Doc(sid).Get(context.TODO())
	if err != nil || docses == nil {
		return ac
	}

	var ss Session = Session{}
	err = docses.DataTo(&ss)
	if err != nil {
		return ac
	}
	var value = ss.Value
	if value["state"].(string) == "InAuth" && value["code_expire"].(int64) > now {

		ac["auth_code"] = value["auth_code"].(string)
		ac["code_challenger"] = value["code_challenger"].(string)
		ac["code_challenger_method"] = value["code_challenger_method"].(string)
	}
	return ac
}

// ActiveSessions returns the number of currently active sessions in the session store
func (pder *SessionProvider) ActiveSessions() int {

	var errcnt, cnt = 0, 0
	var erritr error

	iter := pder.client.Collection(pder.collection).Documents(context.TODO())

	for {
		d, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			errcnt++
			erritr = fmt.Errorf("session id %v, error: %v", d.Ref.ID, err)
		}
		cnt++
	}
	if erritr != nil {
		fmt.Printf("%d errors while counting %d active sessions, error stack: %v", errcnt, cnt, erritr)
	}
	return cnt
}

// Exists check by sid if a session data exists in the session store
func (pder *SessionProvider) Exists(sid string) bool {

	docses, err := pder.client.Collection(pder.collection).Doc(sid).Get(context.TODO())
	if err != nil || docses == nil {
		return false
	}
	return true
}

// Flush will delete all elements for sessions data
func (pder *SessionProvider) Flush() error {

	var erritr error

	iter := pder.client.Collection(pder.collection).Documents(context.TODO())

	for {
		docses, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			erritr = fmt.Errorf("error flushing sessions id: %v, err: %v", docses.Ref.ID, err)
		}
		_, err = docses.Ref.Delete(context.TODO())
		if err != nil {
			erritr = fmt.Errorf("while flushing sessions - delete session id: %v, err: %v", docses.Ref.ID, err)
		}
	}
	return erritr
}

// UpdateAuthSession - update state, access and refresh tokens values for auth session
func (pder *SessionProvider) UpdateAuthSession(sid, at, rt, uid string) error {

	_, err := pder.client.Collection(pder.collection).Doc(sid).Update(context.TODO(),
		[]firestore.Update{
			{
				Path:  "Value.at",
				Value: at,
			},
			{
				Path:  "Value.rt",
				Value: rt,
			},
			{
				Path:  "Value.uid",
				Value: uid,
			},
			{
				Path:  "Value.state",
				Value: "Authed",
			},
		})
	if err != nil {
		return fmt.Errorf("err while updating new authenticated session id %v, err: %v", sid, err)
	}
	return nil
}

// NewSession creates a new session value in the store with sid as a key
func (pder *SessionProvider) NewSession(sid string) (ivmsesman.SessionStore, error) {

	v := make(map[string]interface{})
	v["state"] = "New"

	newsess := Session{Sid: sid, TimeAccessed: time.Now().Unix(), Value: v}

	_, err := pder.client.Collection(pder.collection).Doc(sid).Set(context.TODO(), newsess)
	if err != nil {
		return nil, fmt.Errorf("unable to save in session repository - error: %v", err)
	}

	return &newsess, nil
}

// NewSession creates a new session value in the store with sid as a key
func (pder *SessionProvider) Blacklisting(ip, path string, data interface{}) {

	v := make(map[string]interface{})
	v["created"] = time.Now()
	v["requestURI"] = path
	v["details"] = data

	_, err := pder.client.Collection(pder.blacklist).Doc(ip).Set(context.TODO(), v, firestore.MergeAll)
	if err != nil {
		fmt.Printf("error update ip %s in the blacklist", ip)
		return
	}
	fmt.Printf("ip %s listed in the blacklist", ip)
}

// IsIPExistInBL returns boolean result for the @ip being or not in the blacklist
func (pder *SessionProvider) IsIPExistInBL(ip string) bool {

	_, err := pder.client.Collection(pder.blacklist).Doc(ip).Get(context.TODO())
	return err == nil
}

// init - Initiates at run-time the following code
func init() {
	// Initialize the GCP project to be used
	projectID := os.Getenv("FIRESTORE_PROJECT_ID")
	scn := os.Getenv("SESSION_COLLECTION_NAME")

	client, err := firestore.NewClient(context.TODO(), projectID)
	if err != nil || projectID == "" {
		fmt.Printf("FATAL: firestore client init error %v", err.Error())
		os.Exit(1)
	}
	// defer client.Close()
	pder.client = client
	if scn != "" {
		pder.collection = scn
	}

	ivmsesman.RegisterProvider(ivmsesman.Firestore, pder)
}
