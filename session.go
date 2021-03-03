// This is session module: don't forget to import _ "gaozs/session/provider"
//
// use NewManager(provideName, cookieName, maxlifetime) for get a globe smgr
// use smgr.SessionStart() in each request handle to get related session

// some codes are from beego session module, thx!
package session

import (
	"fmt"
	"net/http"
	"time"
)

type SessionManager struct {
	provider    SessionProvider
	cookieName  string // private cookiename to store session id on client
	maxlifetime int64  // life time for cookie and session: seconds, 0 mean never expire
}

var provides = make(map[string]SessionProvider)

//	maxlifetime: seconds, 0 mean never expire
type SessionProvider interface {
	ProvideInit(maxlifetime int64)                                                  // set session live time: seconds, 0 mean never expire
	SessionID() (string, error)                                                     // generate a new session ID, must be safe to URL query
	SessionGet(sid string, w http.ResponseWriter, r *http.Request) (Session, error) // get session by sid, create a new session if sid is not exist
	SessionDestroy(sid string, w http.ResponseWriter, r *http.Request) error        // delete a sessoion
	SessionGC()                                                                     // loop to do session GC
}

type Session interface {
	Set(key, value interface{}) error // set session value
	Get(key interface{}) interface{}  // get session value
	Delete(key interface{}) error     // delete session value
	Release()                         // call at last to make sure save session data(for cookie)
	SessionID() string                // back current sessionID
}

// Register makes a session provide available by the provided name.
// If Register is called twice with the same name or if driver is nil,
// it panics.
func Register(name string, provider SessionProvider) {
	if provider == nil {
		panic("session: Register provide is nil")
	}
	if _, dup := provides[name]; dup {
		panic("session: Register called twice for provide " + name)
	}
	provides[name] = provider
}

func NewManager(provideName, cookieName string, maxlifetime int64) (*SessionManager, error) {
	provider, ok := provides[provideName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provide %q (forgotten import?)", provideName)
	}
	if maxlifetime < 0 {
		maxlifetime = 0
	}
	smgr := &SessionManager{provider: provider, cookieName: cookieName, maxlifetime: maxlifetime}
	smgr.provider.ProvideInit(maxlifetime)
	smgr.provider.SessionGC()
	return smgr, nil
}

// In a http request handle func, use this to get related session
func (manager *SessionManager) SessionStart(w http.ResponseWriter, r *http.Request) (session Session, err error) {
	cookie, err := r.Cookie(manager.cookieName)
	var sid string
	if err != nil || cookie.Value == "" {
		sid, err = manager.provider.SessionID()
		if err != nil {
			return nil, err
		}
	} else {
		sid = cookie.Value
	}

	session, err = manager.provider.SessionGet(sid, w, r)
	if err != nil {
		return nil, err
	}

	cookie = &http.Cookie{
		Name:     manager.cookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(manager.maxlifetime),
		Expires:  time.Now().Add(time.Duration(manager.maxlifetime) * time.Second),
	}
	http.SetCookie(w, cookie)

	return
}

func (manager *SessionManager) SessionDestroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(manager.cookieName)
	if err != nil || cookie.Value == "" {
		return
	} else {
		manager.provider.SessionDestroy(cookie.Value, w, r)
		cookie := &http.Cookie{
			Name:     manager.cookieName,
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now(),
			MaxAge:   -1,
		}
		http.SetCookie(w, cookie)
	}
}
