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

	// firestore "cloud.google.com/go/firestore/apiv1beta1"
	"github.com/dasiyes/ivmsesman"
)

var pder = &SessionStoreProvider{collection: "sessions"}

// SessionStore defines the storage to store the session data in
type SessionStore struct {
	Sid          string
	TimeAccessed time.Time
	Value        map[interface{}]interface{}
}

// Set stores the key:value pair in the repository
func (st *SessionStore) Set(key, value interface{}) error {
	st.Value[key] = value
	pder.UpdateTimeAccessed(st.Sid)
	return nil
}

// Get will retrieve the session value by the provided key
func (st *SessionStore) Get(key interface{}) interface{} {
	_ = pder.UpdateTimeAccessed(st.Sid)
	if v, ok := st.Value[key]; ok {
		return v
	}
	return nil
}

// Delete will remove a session value by the provided key
func (st *SessionStore) Delete(key interface{}) error {
	delete(st.Value, key)
	pder.UpdateTimeAccessed(st.Sid)
	return nil
}

// SessionID will retrieve the id of the current session
func (st *SessionStore) SessionID() string {
	return st.Sid
}

// GetLTA will return the LastTimeAccessedAt
func (st *SessionStore) GetLTA() time.Time {
	return st.TimeAccessed
}

// SessionStoreProvider ensures storing sessions data
type SessionStoreProvider struct {
	client     *firestore.Client
	collection string
}

// NewSession creates a new session value in the store with sid as a key
func (pder *SessionStoreProvider) NewSession(sid string) (ivmsesman.IvmSS, error) {

	v := make(map[interface{}]interface{})
	newsess := &SessionStore{Sid: sid, TimeAccessed: time.Now(), Value: v}

	wrtRsl, err := pder.client.Collection(pder.collection).Doc(sid).Set(context.TODO(), newsess)
	if err != nil {
		return nil, fmt.Errorf("unable to save in session repository - error: %v", err)
	}
	fmt.Printf("wrtRsl: %v\n", wrtRsl)
	return newsess, nil
}

// FindOrCreate will first search the store for a session value with provided sid. If not not found, a new session value will be created and stored in the session store
func (pder *SessionStoreProvider) FindOrCreate(sid string) (ivmsesman.IvmSS, error) {

	var ss SessionStore

	docses, err := pder.client.Collection(pder.collection).Doc(sid).Get(context.TODO())
	if err != nil {
		if strings.Contains(err.Error(), "Missing or insufficient permissions") {
			return nil, errors.New("insufficient permissions to read data from the session store")
		} else {
			fmt.Printf("err while read session id: %v, err: %v\n", sid, err.Error())
			if !docses.Exists() {
				return pder.NewSession(sid)
			}
			return nil, err
		}
	}

	err = docses.DataTo(ss)
	if err != nil {
		return nil, err
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

	fmt.Println("All expired sessions (deleting...)")
	iter := pder.client.Collection(pder.collection).Where("TimeAccessed", "<", (time.Now().Unix() - maxlifetime)).Documents(context.TODO())

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("err while iterate expired sessions: %v\n", err.Error())
			return
		}
		_, err = doc.Ref.Delete(context.TODO())
		if err != nil {
			fmt.Printf("error deleting session id %v, err: %v\n", doc.Ref.ID, err.Error())
		}
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
		fmt.Printf("err while updating time accessed for sessions id %v, err: %v\n", sid, err.Error())
		return err
	}
	return nil
}

// ActiveSessions returns the number of currently active sessions in the session store
func (pder *SessionStoreProvider) ActiveSessions() int {

	var errcnt, cnt = 0, 0

	iter := pder.client.Collection(pder.collection).Documents(context.TODO())

	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			errcnt++
			fmt.Printf("%v error(s) counting sessions: %v\n", errcnt, err.Error())
		}
		cnt++
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

	var err error

	iter := pder.client.Collection(pder.collection).Documents(context.TODO())

	for {
		docses, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("error flushing sessions: %v\n", err.Error())
		}
		_, err = docses.Ref.Delete(context.TODO())
		if err != nil {
			fmt.Printf("while flushing sessions - delete session id: %v, err: %v\n", docses.Ref.ID, err.Error())
		}
	}
	return err
}

func init() {
	// Initialize the GCP project to be used
	projectID := os.Getenv("PROJECT_ID")
	client, err := firestore.NewClient(context.TODO(), projectID)
	if err != nil {
		fmt.Printf("FATAL: firestore client init error %v", err.Error())
		os.Exit(1)
	}
	defer client.Close()
	pder.client = client
	ivmsesman.RegisterProvider(ivmsesman.Firestore, pder)
}
