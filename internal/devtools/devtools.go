// Package devtools contains local-only helpers for demo and E2E data.
package devtools

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"delim/internal/gateway/auth"
	coreclient "delim/internal/gateway/client/core"
	corev1 "delim/pkg/gen/core/v1"
)

type Actor struct {
	MAXUserID int64
	Name      string
	Token     string
	UserID    int64
}

func RequireLocal(environment string) error {
	if environment != "local" {
		return errors.New("development data is available only when app.env=local")
	}
	return nil
}

func ProvisionActor(
	ctx context.Context,
	client *coreclient.Client,
	sessions *auth.Manager,
	maxUserID int64,
	name string,
	username string,
) (Actor, error) {
	response, err := client.UpsertUser(ctx, &corev1.UpsertUserRequest{
		FirstName: name,
		MaxUserId: maxUserID,
		Username:  username,
	})
	if err != nil {
		return Actor{}, fmt.Errorf("provision %s: %w", name, err)
	}
	userID := response.GetUser().GetId()
	if userID <= 0 {
		return Actor{}, fmt.Errorf("provision %s: Core returned an invalid user", name)
	}
	token, _, err := sessions.Issue(userID, maxUserID)
	if err != nil {
		return Actor{}, fmt.Errorf("issue %s session: %w", name, err)
	}
	return Actor{MAXUserID: maxUserID, Name: name, Token: token, UserID: userID}, nil
}

func LoadLocalEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid environment entry in %s", path)
		}
		key = strings.TrimSpace(key)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func EnvOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
