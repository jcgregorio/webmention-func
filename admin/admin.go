package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	CLIENT_ID = "952643138919-jh0117ivtbqkc9njoh91csm7s465c4na.apps.googleusercontent.com"
)

var (
	ADMINS = []string{"joe.gregorio@gmail.com"}

	client = &http.Client{
		Timeout: time.Second * 10,
	}
)

type Claims struct {
	Mail    string `json:"email"`
	Aud     string `json:"aud"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func IsAdmin(r *http.Request) bool {
	idtoken, err := r.Cookie("id_token")
	if err != nil {
		fmt.Println("Cookie not set.")
		return false
	}
	resp, err := client.Get(fmt.Sprintf("https://www.googleapis.com/oauth2/v3/tokeninfo?%s", idtoken))
	if err != nil || resp.StatusCode != 200 {
		log.Printf("Failed to validate idtoken: %#v %s", *resp, err)
		return false
	}
	claims := Claims{}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		log.Printf("Failed to decode claims: %s", err)
		return false
	}
	// Check if aud is correct.
	if claims.Aud != CLIENT_ID {
		return false
	}

	for _, email := range ADMINS {
		if email == claims.Mail {
			return true
		}
	}
	log.Printf("%q is not an administrator.", claims.Mail)
	return false
}
