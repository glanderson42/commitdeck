package secrets

import "github.com/zalando/go-keyring"

type Store interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}
type Native struct{}

func (Native) Get(account string) (string, error) { return keyring.Get("com.commitdeck.app", account) }
func (Native) Set(account, value string) error {
	return keyring.Set("com.commitdeck.app", account, value)
}
func (Native) Delete(account string) error {
	err := keyring.Delete("com.commitdeck.app", account)
	if err == keyring.ErrNotFound {
		return nil
	}
	return err
}
