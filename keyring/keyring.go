package keyring

import (
	"crypto/subtle"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// Keyring defines an interface for accessing keyring items.
type Keyring interface {
	// Get item.
	// Requires Unlock().
	Get(id string) (*Item, error)

	// Set item.
	// Requires Unlock().
	Set(i *Item) error

	// Delete item.
	// Doesn't require Unlock().
	Delete(id string) (bool, error)

	// List items.
	// Requires Unlock().
	// Items with ids that start with "." are not returned by List.
	List(opts *ListOpts) ([]*Item, error)

	// IDs.
	// Doesn't require Unlock().
	// Items with ids that start with "." are not returned by IDs.
	IDs(prefix string) ([]string, error)

	// Exists returns true it has the id.
	// Doesn't require Unlock().
	Exists(id string) (bool, error)

	// Unlock with auth.
	Unlock(auth Auth) error

	// Lock.
	Lock() error

	// Salt is default salt value, generated on first access and persisted
	// until ResetAuth() or Reset().
	// This salt value is not encrypted in the keyring.
	// Doesn't require Unlock().
	Salt() ([]byte, error)

	// Authed returns true if Keyring has ever been unlocked.
	// Doesn't require Unlock().
	Authed() (bool, error)

	// // ResetAuth resets auth (leaving any keyring items).
	// // Doesn't require Unlock().
	// ResetAuth() error

	// Reset keyring.
	// Doesn't require Unlock().
	Reset() error
}

// ListOpts ...
type ListOpts struct {
	Types []string
}

type store interface {
	get(service string, id string) ([]byte, error)
	set(service string, id string, data []byte, typ string) error
	remove(service string, id string) (bool, error)

	ids(service string, prefix string, showHidden bool, showReserved bool) ([]string, error)
	exists(service string, id string) (bool, error)
}

func getItem(st store, service string, id string, key SecretKey) (*Item, error) {
	if key == nil {
		return nil, ErrLocked
	}
	b, err := st.get(service, id)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	return decodeItem(b, key)
}

func setItem(st store, service string, item *Item, key SecretKey) error {
	if key == nil {
		return ErrLocked
	}
	data, err := item.Marshal(key)
	if err != nil {
		return err
	}
	return st.set(service, item.ID, []byte(data), item.Type)
}

// ErrNotAnItem if value in keyring is not an encoded keyring item.
// TODO: Add test.
var ErrNotAnItem = errors.New("not an encoded keyring item")

func decodeItem(b []byte, key SecretKey) (*Item, error) {
	if b == nil {
		return nil, nil
	}
	if !isItem(b) {
		return nil, ErrNotAnItem
	}
	item, err := DecodeItem(b, key)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func unlock(st store, service string, auth Auth) (SecretKey, error) {
	if auth == nil {
		return nil, errors.Errorf("no auth specified")
	}

	key := auth.Key()

	item, err := getItem(st, service, reserved("auth"), key)
	if err != nil {
		return nil, err
	}
	if item == nil {
		err := setItem(st, service, NewItem(reserved("auth"), NewSecret(key[:]), ""), key)
		if err != nil {
			return nil, err
		}
	} else {
		if subtle.ConstantTimeCompare(item.SecretData(), key[:]) != 1 {
			return nil, errors.Errorf("invalid auth")
		}
	}

	return key, nil
}

func salt(st store, service string) ([]byte, error) {
	salt, err := st.get(service, reserved("salt"))
	if err != nil {
		return nil, err
	}
	if salt == nil {
		salt = rand32()[:]
		if err := st.set(service, reserved("salt"), salt, ""); err != nil {
			return nil, err
		}
	}
	return salt, nil
}

func newKeyring(st store, service string) (*keyring, error) {
	return &keyring{st: st, service: service}, nil
}

var _ Keyring = &keyring{}

type keyring struct {
	st      store
	service string
	key     SecretKey
}

const reservedPrefix = "#"

func reserved(s string) string {
	return reservedPrefix + s
}

const hiddenPrefix = "."

func hidden(s string) string {
	return hiddenPrefix + s
}

func (k *keyring) Get(id string) (*Item, error) {
	if strings.HasPrefix(id, reservedPrefix) {
		return nil, errors.Errorf("keyring id prefix reserved %s", id)
	}
	return getItem(k.st, k.service, id, k.key)
}

func (k *keyring) Set(item *Item) error {
	if item.ID == "" {
		return errors.Errorf("no id")
	}
	if strings.HasPrefix(item.ID, reservedPrefix) {
		return errors.Errorf("keyring id prefix reserved %s", item.ID)
	}
	return setItem(k.st, k.service, item, k.key)
}

func (k *keyring) Delete(id string) (bool, error) {
	return k.st.remove(k.service, id)
}

func (k *keyring) List(opts *ListOpts) ([]*Item, error) {
	if opts == nil {
		opts = &ListOpts{}
	}
	if k.key == nil {
		return nil, ErrLocked
	}

	ids, err := k.st.ids(k.service, "", false, false)
	if err != nil {
		return nil, err
	}
	items := make([]*Item, 0, len(ids))
	for _, id := range ids {
		b, err := k.st.get(k.service, id)
		if err != nil {
			return nil, err
		}
		item, err := DecodeItem(b, k.key)
		if err != nil {
			return nil, err
		}
		if len(opts.Types) != 0 && !contains(opts.Types, item.Type) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (k *keyring) IDs(prefix string) ([]string, error) {
	return k.st.ids(k.service, prefix, false, false)
}

func (k *keyring) Exists(id string) (bool, error) {
	return k.st.exists(k.service, id)
}

func (k *keyring) Unlock(auth Auth) error {
	key, err := unlock(k.st, k.service, auth)
	if err != nil {
		return err
	}
	k.key = key
	return nil
}

func (k *keyring) Lock() error {
	k.key = nil
	return nil
}

func (k *keyring) Salt() ([]byte, error) {
	return salt(k.st, k.service)
}

func (k *keyring) Authed() (bool, error) {
	return k.st.exists(k.service, reserved("auth"))
}

// func (k *keyring) ResetAuth() error {
// 	if _, err := k.st.remove(k.service, reserved("salt")); err != nil {
// 		return err
// 	}
// 	if _, err := k.st.remove(k.service, reserved("auth")); err != nil {
// 		return err
// 	}
// 	return nil
// }

func (k *keyring) Reset() error {
	ids, err := k.st.ids(k.service, "", true, true)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := k.st.remove(k.service, id); err != nil {
			return err
		}
	}
	return k.Lock()
}

func contains(strs []string, s string) bool {
	for _, e := range strs {
		if e == s {
			return true
		}
	}
	return false
}
