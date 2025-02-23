package mongostore

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/qiniu/qmgo"
)

var (
	ErrInvalidId = errors.New("mgostore: invalid session id")
)

// Session object store in MongoDB
type Session struct {
	Id       string `bson:"_id,omitempty"`
	Data     string
	Modified time.Time
}

// MongoStore stores sessions in MongoDB
type MongoStore struct {
	Codecs  []securecookie.Codec
	Options *sessions.Options
	Token   TokenGetSeter
	qc      *qmgo.QmgoClient
}

// NewMongoStore returns a new MongoStore.
// Set ensureTTL to true let the database auto-remove expired object by maxAge.
func NewMongoStore(qc *qmgo.QmgoClient, maxAge int, domain string,
	keyPairs ...[]byte) *MongoStore {
	store := &MongoStore{
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		Options: &sessions.Options{
			Path:   "/",
			MaxAge: maxAge,
		},
		Token: &CookieToken{},
		qc:    qc,
	}
	if len(domain) > 0 {
		store.Options.Domain = domain
	}

	store.MaxAge(maxAge)

	// don't add index in codes
	//if ensureTTL {
	//	c.EnsureIndex(mgo.Index{
	//		Key:         []string{"modified"},
	//		Background:  true,
	//		Sparse:      true,
	//		ExpireAfter: time.Duration(maxAge) * time.Second,
	//	})
	//}

	return store
}

// Get registers and returns a session for the given name and session store.
// It returns a new session if there are no sessions registered for the name.
func (m *MongoStore) Get(r *http.Request, name string) (
	*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(m, name)
}

// CreateNew returns a new session for the given name without adding it to the registry.
func (m *MongoStore) CreateNew(r *http.Request, name string) (
	*sessions.Session, error) {
	session := sessions.NewSession(m, name)
	session.Options = &sessions.Options{
		Path:     m.Options.Path,
		MaxAge:   m.Options.MaxAge,
		Domain:   m.Options.Domain,
		Secure:   m.Options.Secure,
		HttpOnly: m.Options.HttpOnly,
	}
	session.IsNew = true
	return session, nil
}

// New returns a session for the given name without adding it to the registry.
func (m *MongoStore) New(r *http.Request, name string) (
	*sessions.Session, error) {
	session := sessions.NewSession(m, name)
	session.Options = &sessions.Options{
		Path:     m.Options.Path,
		MaxAge:   m.Options.MaxAge,
		Domain:   m.Options.Domain,
		Secure:   m.Options.Secure,
		HttpOnly: m.Options.HttpOnly,
	}
	session.IsNew = true
	var err error
	if cook, errToken := m.Token.GetToken(r, name); errToken == nil {
		err = securecookie.DecodeMulti(name, cook, &session.ID, m.Codecs...)
		if err == nil {
			err = m.load(session)
			if err == nil {
				session.IsNew = false
			} else {
				err = nil
			}
		}
	}
	return session, err
}

// Save saves all sessions registered for the current request.
func (m *MongoStore) Save(r *http.Request, w http.ResponseWriter,
	session *sessions.Session) error {
	if session.Options.MaxAge < 0 {
		if err := m.delete(session); err != nil {
			return err
		}
		m.Token.SetToken(w, session.Name(), "", session.Options)
		return nil
	}

	if session.ID == "" {
		session.ID = primitive.NewObjectID().Hex()
	}

	if err := m.upsert(session); err != nil {
		return err
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID,
		m.Codecs...)
	if err != nil {
		return err
	}

	m.Token.SetToken(w, session.Name(), encoded, session.Options)
	return nil
}

// MaxAge sets the maximum age for the store and the underlying cookie
// implementation. Individual sessions can be deleted by setting Options.MaxAge
// = -1 for that session.
func (m *MongoStore) MaxAge(age int) {
	m.Options.MaxAge = age

	// Set the maxAge for each securecookie instance.
	for _, codec := range m.Codecs {
		if sc, ok := codec.(*securecookie.SecureCookie); ok {
			sc.MaxAge(age)
		}
	}
}

func (m *MongoStore) load(session *sessions.Session) error {

	if _, err := primitive.ObjectIDFromHex(session.ID); err != nil {
		return ErrInvalidId
	}
	ctx, fn := context.WithTimeout(context.Background(), 5*time.Second)
	defer fn()
	s := Session{}
	err := m.qc.Find(ctx, bson.M{"_id": session.ID}).One(&s)
	if err != nil {
		return err
	}

	if err := securecookie.DecodeMulti(session.Name(), s.Data, &session.Values,
		m.Codecs...); err != nil {
		return err
	}

	return nil
}

func (m *MongoStore) upsert(session *sessions.Session) error {
	if _, err := primitive.ObjectIDFromHex(session.ID); err != nil {
		return ErrInvalidId
	}

	var modified time.Time
	if val, ok := session.Values["modified"]; ok {
		modified, ok = val.(time.Time)
		if !ok {
			return errors.New("mongostore: invalid modified value")
		}
	} else {
		modified = time.Now()
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values,
		m.Codecs...)
	if err != nil {
		return err
	}

	s := Session{
		Id:       session.ID,
		Data:     encoded,
		Modified: modified,
	}
	ctx, fn := context.WithTimeout(context.Background(), 5*time.Second)
	defer fn()
	_, err = m.qc.UpsertId(ctx, s.Id, &s)
	if err != nil {
		return err
	}

	return nil
}

func (m *MongoStore) delete(session *sessions.Session) error {
	if _, err := primitive.ObjectIDFromHex(session.ID); err != nil {
		return ErrInvalidId
	}
	ctx, fn := context.WithTimeout(context.Background(), 5*time.Second)
	defer fn()
	return m.qc.RemoveId(ctx, session.ID)
}

func (m *MongoStore) Delete(w http.ResponseWriter, session *sessions.Session) error {
	if err := m.delete(session); err != nil {
		return err
	}
	//m.Token.SetToken(w, session.Name(), "", session.Options)
	return nil
}
