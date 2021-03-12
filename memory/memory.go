package memory

import (
	"container/list"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"gaozs.ddns.net/go/session"
)

var pder = &Provider{list: list.New()}

// implment session.Session interface
type SessionStore struct {
	sid          string                      // session id
	timeAccessed time.Time                   // last access time
	value        map[interface{}]interface{} // session key-values
}

func (st *SessionStore) Set(key, value interface{}) error {
	pder.sessionUpdate(st.sid)
	st.value[key] = value
	return nil
}

func (st *SessionStore) Get(key interface{}) interface{} {
	pder.sessionUpdate(st.sid)
	if v, ok := st.value[key]; ok {
		return v
	}
	return nil
}

func (st *SessionStore) Delete(key interface{}) error {
	pder.sessionUpdate(st.sid)
	delete(st.value, key)
	return nil
}

func (st *SessionStore) Release() {
	// no use for memory session
}

func (st *SessionStore) SessionID() string {
	return st.sid
}

//Implement session.SessionProvider interface
type Provider struct {
	lock        sync.Mutex               // use to protect memory
	maxlifetime int64                    // life time session: seconds, 0 mean never expire
	sessions    map[string]*list.Element // sid-sessiondata map, each session is a List.Element's value
	list        *list.List               // used for quick gc
}

func (pder *Provider) ProvideInit(maxlifetime int64) {
	pder.lock.Lock()
	defer pder.lock.Unlock()
	pder.maxlifetime = maxlifetime
}

func (pder *Provider) SessionID() (string, error) {
	b := make([]byte, 32)
	n, err := rand.Read(b)
	if n != len(b) || err != nil {
		return "", fmt.Errorf("Could not successfully read from the system CSPRNG.")
	}
	return hex.EncodeToString(b), nil
}

func (pder *Provider) sessionInit(sid string) (session.Session, error) {
	pder.lock.Lock()
	defer pder.lock.Unlock()
	v := make(map[interface{}]interface{}, 0)                              // session's value(key-values map)
	newsess := &SessionStore{sid: sid, timeAccessed: time.Now(), value: v} // create a new session
	element := pder.list.PushFront(newsess)                                // insert new session into list last
	pder.sessions[sid] = element                                           // for quick get session
	return newsess, nil
}

func (pder *Provider) SessionGet(sid string, w http.ResponseWriter, r *http.Request) (session.Session, error) {
	if element, ok := pder.sessions[sid]; ok {
		return element.Value.(*SessionStore), nil // do a type assert to ensure element's value is *SessionStore
	}
	sess, err := pder.sessionInit(sid) // start a new session
	return sess, err
}

func (pder *Provider) SessionDestroy(sid string, w http.ResponseWriter, r *http.Request) error {
	if element, ok := pder.sessions[sid]; ok {
		delete(pder.sessions, sid)
		pder.list.Remove(element)
		return nil
	}
	return nil
}

func (pder *Provider) SessionGC() {
	if pder.maxlifetime <= 0 {
		// no need to do GC
		return
	}

	pder.lock.Lock()
	defer pder.lock.Unlock()

	for {
		element := pder.list.Back()
		if element == nil {
			break
		}
		if (element.Value.(*SessionStore).timeAccessed.Unix() + pder.maxlifetime) < time.Now().Unix() {
			pder.list.Remove(element)
			delete(pder.sessions, element.Value.(*SessionStore).sid)
		} else {
			break
		}
	}
	time.AfterFunc(time.Minute*5, func() { pder.SessionGC() })
}

func (pder *Provider) sessionUpdate(sid string) error {
	pder.lock.Lock()
	defer pder.lock.Unlock()
	if element, ok := pder.sessions[sid]; ok {
		element.Value.(*SessionStore).timeAccessed = time.Now() // here do a type asset
		pder.list.MoveToFront(element)
		return nil
	}
	return nil
}

func init() {
	pder.sessions = make(map[string]*list.Element, 0)
	session.Register("memory", pder)
}
