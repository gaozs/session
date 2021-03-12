// some codes are from beego session module, thx!
package cookie

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"gaozs.ddns.net/go/util/session"
	"gaozs.ddns.net/go/util/slog"
)

var pder = &Provider{}

// implment session.Session interface in cookie
type SessionStore struct {
	sid string // session id , also mean cookie name for session data
	w   http.ResponseWriter
	r   *http.Request
	v   map[interface{}]interface{}
}

func (st *SessionStore) initCookieData() {
	cookie, err := st.r.Cookie(st.sid)
	if err != nil || cookie.Value == "" {
		return
	}
	//slog.Debug("get a cookie value: ", cookie.Value)
	cdata, _ := base64.URLEncoding.DecodeString(cookie.Value)
	dec := gob.NewDecoder(bytes.NewReader(cdata))
	dec.Decode(&(st.v))
	slog.Debug("parsed sessData:", st.v)
	return
}

func (st *SessionStore) Set(key, value interface{}) error {
	st.v[key] = value
	return nil
}

func (st *SessionStore) Get(key interface{}) interface{} {
	value, ok := st.v[key]
	if ok {
		return value
	}
	return nil
}

func (st *SessionStore) Delete(key interface{}) error {
	delete(st.v, key)
	return nil
}

// update cookie data to w, should only call once and before any info to w.body, otherwise data may be lost
func (st *SessionStore) Release() {
	cdata := new(bytes.Buffer)
	enc := gob.NewEncoder(cdata)
	err := enc.Encode(st.v)
	if err != nil {
		slog.Fatal(err)
	}
	//slog.Debug("vdata:", st.v)
	//slog.Debug("cdata:", cdata.Bytes())

	cookie := &http.Cookie{
		Name:     st.sid,
		Value:    base64.URLEncoding.EncodeToString(cdata.Bytes()),
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(pder.maxlifetime),
		Expires:  time.Now().Add(time.Duration(pder.maxlifetime) * time.Second),
	}
	//slog.Debugf("to set cookie(%v)\n", *cookie)
	http.SetCookie(st.w, cookie)
}

func (st *SessionStore) SessionID() string {
	return st.sid
}

//Implement session.SessionProvider interface
type Provider struct {
	lock        sync.Mutex // use to protect memory
	maxlifetime int64      // life time session: seconds, 0 mean never expire
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

func (pder *Provider) SessionGet(sid string, w http.ResponseWriter, r *http.Request) (session.Session, error) {
	session := &SessionStore{
		sid: sid,
		w:   w,
		r:   r,
		v:   make(map[interface{}]interface{}),
	}
	session.initCookieData()
	return session, nil
}

func (pder *Provider) SessionDestroy(sid string, w http.ResponseWriter, r *http.Request) error {
	cookie := &http.Cookie{
		Name:     sid,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Now(),
	}
	http.SetCookie(w, cookie)
	return nil
}

func (pder *Provider) SessionGC() {
	// no need to do GC for cookie session
}

func init() {
	session.Register("cookie", pder)
}
