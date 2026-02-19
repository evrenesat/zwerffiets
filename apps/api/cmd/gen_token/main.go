package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	const signingSecret = "test-secret"

	claims := jwt.MapClaims{
		"email": "admin@example.com",
		"role":  "admin",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(8 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(signingSecret))
	if err != nil {
		panic(err)
	}
	fmt.Println(signedToken)
}
