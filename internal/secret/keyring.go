package secret

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	serviceName         = "wabsignal"
	queryTokenUser      = "grafana-http-read-token"
	managementTokenUser = "grafana-cloud-policy-token"
)

var ErrNotFound = errors.New("secret not found")

func GetQueryToken() (string, error) {
	return get(queryTokenUser)
}

func SetQueryToken(token string) error {
	return set(queryTokenUser, token)
}

func DeleteQueryToken() error {
	return del(queryTokenUser)
}

func GetManagementToken() (string, error) {
	return get(managementTokenUser)
}

func SetManagementToken(token string) error {
	return set(managementTokenUser, token)
}

func DeleteManagementToken() error {
	return del(managementTokenUser)
}

func get(user string) (string, error) {
	value, err := keyring.Get(serviceName, user)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrNotFound
	}
	return value, nil
}

func set(user, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return del(user)
	}
	return keyring.Set(serviceName, user, value)
}

func del(user string) error {
	err := keyring.Delete(serviceName, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
