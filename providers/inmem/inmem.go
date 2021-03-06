// Package inmem should be dummy imported with _ "girhub.com/dasiyes/ivmsesman/inmem" in order to auto run init() method which will register the provider in the ivmsesman package by invoking method RegisterProvider
package inmem

import (
	"container/list"
	"sync"
	"time"

	"github.com/dasiyes/ivmsesman"
)

var pder = &SessionStoreProvider{list: list.New()}

// SessionStore defines the storage to store the session data in
type SessionStore struct {
	sid          string
	timeAccessed int64
	value        map[string]interface{}
}

// Set stores the key:value pair in the repository
func (st *SessionStore) Set(key string, value interface{}) error {
	st.value[key] = value
	pder.UpdateTimeAccessed(st.sid)
	return nil
}

// Get will retrieve the session value by the provided key
func (st *SessionStore) Get(key string) interface{} {
	_ = pder.UpdateTimeAccessed(st.sid)
	if v, ok := st.value[key]; ok {
		return v
	}
	return nil
}

// Delete will remove a session value by the provided key
func (st *SessionStore) Delete(key string) error {
	delete(st.value, key)
	pder.UpdateTimeAccessed(st.sid)
	return nil
}

// SessionID will retrieve the id of the current session
func (st *SessionStore) SessionID() string {
	return st.sid
}

// GetLTA will return the LastTimeAccessedAt
func (st *SessionStore) GetLTA() time.Time {
	return time.Unix(st.timeAccessed, 0)
}

// SessionStoreProvider ensures storing sessions data
type SessionStoreProvider struct {
	lock     sync.Mutex
	sessions map[string]*list.Element
	list     *list.List
}

// NewSession creates a new session value in the store with sid as a key
func (pder *SessionStoreProvider) NewSession(sid string) (ivmsesman.IvmSS, error) {

	pder.lock.Lock()
	defer pder.lock.Unlock()

	v := make(map[string]interface{})
	v["state"] = "new"
	newsess := SessionStore{sid: sid, timeAccessed: time.Now().Unix(), value: v}
	element := pder.list.PushBack(newsess)
	pder.sessions[sid] = element
	return &newsess, nil
}

// FindOrCreate will first search the store for a session value with provided sid. If not not found, a new session value will be created and stored in the session store
func (pder *SessionStoreProvider) FindOrCreate(sid string) (ivmsesman.IvmSS, error) {

	if element, ok := pder.sessions[sid]; ok {
		sesel := element.Value.(*SessionStore)
		sesel.timeAccessed = time.Now().Unix()
		return sesel, nil
	}

	sess, err := pder.NewSession(sid)
	return sess, err
}

// Destroy will remove a session data from the storage
func (pder *SessionStoreProvider) Destroy(sid string) error {

	if element, ok := pder.sessions[sid]; ok {
		delete(pder.sessions, sid)
		pder.list.Remove(element)
		return nil
	}
	// TODO: return apropriet error
	return nil
}

// SessionGC cleans all expired sessions
func (pder *SessionStoreProvider) SessionGC(maxlifetime int64) {

	pder.lock.Lock()
	defer pder.lock.Unlock()

	for {
		element := pder.list.Back()
		if element == nil {
			break
		}

		if (element.Value.(*SessionStore).timeAccessed + maxlifetime) < time.Now().Unix() {
			pder.list.Remove(element)
			delete(pder.sessions, element.Value.(*SessionStore).sid)
		} else {
			break
		}
	}
}

// UpdateTimeAccessed will update the time accessed value with now()
func (pder *SessionStoreProvider) UpdateTimeAccessed(sid string) error {

	pder.lock.Lock()
	defer pder.lock.Unlock()

	if element, ok := pder.sessions[sid]; ok {
		element.Value.(*SessionStore).timeAccessed = time.Now().Unix()
		pder.list.MoveToFront(element)
		return nil
	}
	// TODO: return apropriet error
	return nil
}

// ActiveSessions returns the number of currently active sessions in the session store
func (pder *SessionStoreProvider) ActiveSessions() int {

	pder.lock.Lock()
	defer pder.lock.Unlock()

	return len(pder.sessions)

}

// Exists check by sid if a session data exists in the session store
func (pder *SessionStoreProvider) Exists(sid string) bool {

	pder.lock.Lock()
	defer pder.lock.Unlock()

	if _, ok := pder.sessions[sid]; ok {
		return true
	}
	return false
}

// Flush will delete all elements for sessions data
func (pder *SessionStoreProvider) Flush() error {

	pder.lock.Lock()
	defer pder.lock.Unlock()

	pder.list = pder.list.Init()
	return nil
}

func init() {
	pder.sessions = make(map[string]*list.Element)
	ivmsesman.RegisterProvider(ivmsesman.Memory, pder)
}
