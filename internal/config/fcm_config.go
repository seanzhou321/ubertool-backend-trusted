package config

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"

	"google.golang.org/api/option"
)

// InitFirebase initialises the Firebase Messaging client.
// It returns (nil, nil) when keyPath is empty or the credentials file is absent
// so callers can treat a missing file as "push disabled" rather than a fatal error.
func InitFirebase(keyPath string) (*messaging.Client, error) {
	if keyPath == "" {
		keyPath = "config/firebase-admin-key.json"
	}
	opt := option.WithCredentialsFile(keyPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, err
	}
	return app.Messaging(context.Background())
}
