package firestore

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

var pder = &SessionStoreProvider{collection: "sessions"}

// SessionStore defines the document to store the session data in NoSQL db
type SessionStore struct {
	Sid          string
	TimeAccessed int64
	Value        map[string]interface{}
}

// Set stores the key:value pair in the repository
func (st *SessionStore) Set(key, value interface{}) error {
	st.Value[key.(string)] = value
	pder.UpdateTimeAccessed(st.Sid)
	return nil
}

// Get will retrieve the session value by the provided key
func (st *SessionStore) Get(key interface{}) interface{} {
	_ = pder.UpdateTimeAccessed(st.Sid)
	if v, ok := st.Value[key.(string)]; ok {
		return v
	}
	return nil
}

// Delete will remove a session value by the provided key
func (st *SessionStore) Delete(key interface{}) error {
	delete(st.Value, key.(string))
	pder.UpdateTimeAccessed(st.Sid)
	return nil
}

// SessionID will retrieve the id of the current session
func (st *SessionStore) SessionID() string {
	fmt.Printf("Sid: %v", st.TimeAccessed)
	return st.Sid
}

// GetLTA will return the LastTimeAccessedAt
func (st *SessionStore) GetLTA() time.Time {
	return time.Unix(st.TimeAccessed, 0)
}

// SessionStoreProvider ensures storing sessions data
type SessionStoreProvider struct {
	client     *firestore.Client
	collection string
	sessions   map[string]interface{}
}

// NewSession creates a new session value in the store with sid as a key
func (pder *SessionStoreProvider) NewSession(sid string) (ivmsesman.SessionStore, error) {

	v := make(map[string]interface{})
	v["state"] = "New"

	newsess := SessionStore{Sid: sid, TimeAccessed: time.Now().Unix(), Value: v}

	_, err := pder.client.Collection(pder.collection).Doc(sid).Set(context.TODO(), newsess)
	if err != nil {
		return nil, fmt.Errorf("unable to save in session repository - error: %v", err)
	}
	pder.sessions[sid] = newsess

	return &newsess, nil
}

// FindOrCreate will first search the store for a session value with provided sid. If not not found, a new session value will be created and stored in the session store
func (pder *SessionStoreProvider) FindOrCreate(sid string) (ivmsesman.SessionStore, error) {

	var ss SessionStore

	docses, err := pder.client.Collection(pder.collection).Doc(sid).Get(context.TODO())
	if err != nil {
		if strings.Contains(err.Error(), "Missing or insufficient permissions") {
			return nil, errors.New("insufficient permissions to read data from the session store")
		} else {
			if !docses.Exists() {
				return pder.NewSession(sid)
			}
			return nil, fmt.Errorf("err while read session id: %v, err: %v", sid, err)
		}
	}

	err = docses.DataTo(ss)
	if err != nil {
		return nil, fmt.Errorf("error while converting firstore doc to session object: %v", err)
	}

	return &ss, nil
}

// Destroy will remove a session data from the storage
func (pder *SessionStoreProvider) Destroy(sid string) error {

	_, err := pder.client.Collection(pder.collection).Doc(sid).Delete(context.TODO())
	if err != nil {
		return err
	}
	return nil
}

// SessionGC cleans all expired sessions
func (pder *SessionStoreProvider) SessionGC(maxlifetime int64) {

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

// UpdateTimeAccessed will update the time accessed value with now()
func (pder *SessionStoreProvider) UpdateTimeAccessed(sid string) error {
	_, err := pder.client.Collection(pder.collection).Doc(sid).Update(context.TODO(),
		[]firestore.Update{
			{
				Path:  "TimeAccessed",
				Value: firestore.ServerTimestamp,
			},
		})
	if err != nil {
		return fmt.Errorf("err while updating time accessed for sessions id %v, err: %v", sid, err)
	}
	return nil
}

// ActiveSessions returns the number of currently active sessions in the session store
func (pder *SessionStoreProvider) ActiveSessions() int {

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
func (pder *SessionStoreProvider) Exists(sid string) bool {

	docses, err := pder.client.Collection(pder.collection).Doc(sid).Get(context.TODO())
	if err != nil || !docses.Exists() {
		return false
	}
	return true
}

// Flush will delete all elements for sessions data
func (pder *SessionStoreProvider) Flush() error {

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

func init() {
	// Initialize the GCP project to be used
	projectID := os.Getenv("PROJECT_ID")
	client, err := firestore.NewClient(context.TODO(), projectID)
	if err != nil {
		fmt.Printf("FATAL: firestore client init error %v", err.Error())
		os.Exit(1)
	}
	// defer client.Close()
	pder.client = client
	ivmsesman.RegisterProvider(ivmsesman.Firestore, pder)
}
