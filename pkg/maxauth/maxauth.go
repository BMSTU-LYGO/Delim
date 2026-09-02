package maxauth

type Verifier struct {
	secret string
}

func New(secret string) *Verifier {
	return &Verifier{secret: secret}
}
