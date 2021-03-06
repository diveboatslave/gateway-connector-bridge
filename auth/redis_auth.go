// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package auth

import (
	"errors"
	"time"

	redis "gopkg.in/redis.v5"
)

// Redis implements the authentication interface with a Redis backend
type Redis struct {
	prefix string
	client *redis.Client
	Exchanger
}

// DefaultRedisPrefix is used as prefix when no prefix is given
var DefaultRedisPrefix = "gateway:"

var redisKey = struct {
	token        string
	key          string
	tokenExpires string
}{
	token:        "token",
	key:          "key",
	tokenExpires: "token_expires",
}

// NewRedis returns a new authentication interface with a redis backend
func NewRedis(client *redis.Client, prefix string) Interface {
	if prefix == "" {
		prefix = DefaultRedisPrefix
	}
	return &Redis{
		client: client,
		prefix: prefix,
	}
}

// SetToken sets the access token for a gateway
func (r *Redis) SetToken(gatewayID string, token string, expires time.Time) error {
	data := map[string]string{
		redisKey.token: token,
	}
	if expires.IsZero() {
		data[redisKey.tokenExpires] = ""
	} else {
		data[redisKey.tokenExpires] = expires.Format(time.RFC3339)
	}
	return r.client.HMSet(r.prefix+gatewayID, data).Err()
}

// SetKey sets the access key for a gateway
func (r *Redis) SetKey(gatewayID string, key string) error {
	return r.client.HSet(r.prefix+gatewayID, redisKey.key, key).Err()
}

// ValidateKey validates the access key for a gateway
func (r *Redis) ValidateKey(gatewayID string, key string) error {
	res, err := r.client.HGet(r.prefix+gatewayID, redisKey.key).Result()
	if err == redis.Nil || len(res) == 0 || key == res {
		return nil
	}
	return errors.New("Invalid Key")
}

// Delete gateway key and token
func (r *Redis) Delete(gatewayID string) error {
	return r.client.Del(r.prefix + gatewayID).Err()
}

// GetToken returns an access token for the gateway
func (r *Redis) GetToken(gatewayID string) (string, error) {
	res, err := r.client.HGetAll(r.prefix + gatewayID).Result()
	if err == redis.Nil || len(res) == 0 {
		return "", ErrGatewayNotFound
	}
	if err != nil {
		return "", err
	}
	var expires time.Time
	if expiresStr, ok := res[redisKey.tokenExpires]; ok && expiresStr != "" {
		if rExpires, err := time.Parse(time.RFC3339, expiresStr); err == nil {
			expires = rExpires
		}
	}
	if token, ok := res[redisKey.token]; ok && token != "" {
		if expires.IsZero() || expires.After(time.Now()) {
			return token, nil
		}
	}
	if key, ok := res[redisKey.key]; ok && key != "" && r.Exchanger != nil {
		token, expires, err := r.Exchange(gatewayID, key)
		if err != nil {
			return "", err
		}
		if err := r.SetToken(gatewayID, token, expires); err != nil {
			// TODO: Print warning
		}
		return token, nil
	}
	return "", ErrGatewayNoValidToken
}

// SetExchanger sets the component that will exchange access keys for access tokens
func (r *Redis) SetExchanger(e Exchanger) {
	r.Exchanger = e
}
