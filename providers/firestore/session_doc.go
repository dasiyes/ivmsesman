package firestoredb

import (
	"fmt"
	"time"
)

// Session  represents a single document (session) in the database and
// provides the operations methods to handle its attributes once the document
// is provided
type Session struct {
	Sid          string
	TimeAccessed int64
	Value        map[string]interface{}
}

// Set stores the key:value pair in the repository
func (st *Session) Set(key, value interface{}) error {
	st.Value[key.(string)] = value
	pder.UpdateTimeAccessed(st.Sid)
	return nil
}

// Get will retrieve the session value by the provided key
func (st *Session) Get(key interface{}) interface{} {
	_ = pder.UpdateTimeAccessed(st.Sid)
	if v, ok := st.Value[key.(string)]; ok {
		return v
	}
	return nil
}

// Delete will remove a session value by the provided key
func (st *Session) Delete(key interface{}) error {
	delete(st.Value, key.(string))
	pder.UpdateTimeAccessed(st.Sid)
	return nil
}

// SessionID will retrieve the id of the current session
func (st *Session) SessionID() string {
	fmt.Printf("Sid: %v, Last Accessed: %d\n", st.Sid, st.TimeAccessed)
	return st.Sid
}

// GetLTA will return the LastTimeAccessedAt
func (st *Session) GetLTA() time.Time {
	return time.Unix(st.TimeAccessed, 0)
}
