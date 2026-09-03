package maxauth

import "crypto/hmac"

type WebhookVerifier struct {
	secret string
}

func NewWebhookVerifier(secret string) *WebhookVerifier {
	return &WebhookVerifier{secret: secret}
}

func (v *WebhookVerifier) Verify(secret string) bool {
	return hmac.Equal([]byte(v.secret), []byte(secret))
}
