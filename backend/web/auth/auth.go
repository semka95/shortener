package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v4"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

// KeyLookupFunc is used to map a JWT key id (kid) to the corresponding public key.
// It is a requirement for creating an Authenticator.
//
// * Private keys should be rotated. During the transition period, tokens
// signed with the old and new keys can coexist by looking up the correct
// public key by key id (kid).
//
// * Key-id-to-public-key resolution is usually accomplished via a public JWKS
// endpoint. See https://auth0.com/docs/jwks for more details.
type KeyLookupFunc func(kid string) (*rsa.PublicKey, error)

// NewSimpleKeyLookupFunc is a simple implementation of KeyFunc that only ever
// supports one key. This is easy for development but in production should be
// replaced with a caching layer that calls a JWKS endpoint.
func NewSimpleKeyLookupFunc(activeKID string, publicKey *rsa.PublicKey) KeyLookupFunc {
	f := func(kid string) (*rsa.PublicKey, error) {
		if activeKID != kid {
			return nil, fmt.Errorf("unrecognized key id %q", kid)
		}
		return publicKey, nil
	}

	return f
}

// Authenticator is used to authenticate clients. It can generate a token for a
// set of user claims and recreate the claims by parsing the token.
type Authenticator struct {
	JWTConfig        echojwt.Config
	privateKey       *rsa.PrivateKey
	activeKID        string
	algorithm        string
	pubKeyLookupFunc KeyLookupFunc
	parser           *jwt.Parser
}

// NewAuthenticator creates an *Authenticator for use. It will error if:
// - The private key is nil.
// - The public key func is nil.
// - The key ID is blank.
// - The specified algorithm is unsupported.
func NewAuthenticator(privateKey *rsa.PrivateKey, activeKID, algorithm string, publicKeyLookupFunc KeyLookupFunc) (*Authenticator, error) {
	if privateKey == nil {
		return nil, errors.New("private key can't be nil")
	}
	if activeKID == "" {
		return nil, errors.New("active kid can't be blank")
	}
	if jwt.GetSigningMethod(algorithm) == nil {
		return nil, fmt.Errorf("unknown algorithm %v", algorithm)
	}
	if publicKeyLookupFunc == nil {
		return nil, errors.New("public key function can't be nil")
	}

	// Create the token parser to use. The algorithm used to sign the JWT must be
	// validated to avoid a critical vulnerability:
	// https://auth0.com/blog/critical-vulnerabilities-in-json-web-token-libraries/
	parser := jwt.Parser{
		ValidMethods: []string{algorithm},
	}

	jwtConfig := echojwt.Config{
		SigningMethod: algorithm,
		SigningKey:    privateKey.Public().(*rsa.PublicKey),
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(Claims)
		},
	}

	a := Authenticator{
		JWTConfig:        jwtConfig,
		privateKey:       privateKey,
		activeKID:        activeKID,
		algorithm:        algorithm,
		pubKeyLookupFunc: publicKeyLookupFunc,
		parser:           &parser,
	}

	return &a, nil
}

// GenerateToken generates a signed JWT token string representing the user Claims.
func (a *Authenticator) GenerateToken(claims *Claims) (string, error) {
	method := jwt.GetSigningMethod(a.algorithm)

	tkn := jwt.NewWithClaims(method, claims)
	tkn.Header["kid"] = a.activeKID

	str, err := tkn.SignedString(a.privateKey)
	if err != nil {
		return "", fmt.Errorf("can't sign token: %w", err)
	}

	return str, nil
}
