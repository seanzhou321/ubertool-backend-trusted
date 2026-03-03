package config

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"

	"google.golang.org/api/option"
)

func initFirebase() (*messaging.Client, error) {
	opt := option.WithCredentialsFile("config/fcmServiceAccountKey.json")
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, err
	}
	return app.Messaging(context.Background())
}

// InitFirebase initialises the Firebase Messaging client.
// It returns (nil, nil) when the credentials file is absent so callers
// can treat a missing file as "push disabled" rather than a fatal error.
func InitFirebase() (*messaging.Client, error) {
	return initFirebase()
}
