package maxupdate

import "testing"

func TestFormatMoneyMinor(t *testing.T) {
	cases := []struct {
		minor    int64
		currency string
		want     string
	}{
		{124000, "RUB", "1 240 ₽"},
		{85000, "RUB", "850 ₽"},
		{100, "RUB", "1 ₽"},
		{150, "RUB", "1,50 ₽"},
		{105, "RUB", "1,05 ₽"},
		{1234560000, "RUB", "12 345 600 ₽"},
		{2500, "KZT", "25 KZT"},
	}
	for _, entry := range cases {
		if got := formatMoneyMinor(entry.minor, entry.currency); got != entry.want {
			t.Errorf("formatMoneyMinor(%d,%q) = %q, want %q", entry.minor, entry.currency, got, entry.want)
		}
	}
}

func TestOpenAppKeyboardBuildsLaunchLink(t *testing.T) {
	d := &Dispatcher{miniAppURL: "https://app.example/"}
	keyboard := d.openAppKeyboard("Открыть баланс", "dl_token")
	button := keyboard.Payload.Buttons[0][0]
	if button.Type != "open_app" {
		t.Fatalf("button type = %q, want open_app", button.Type)
	}
	if button.URL != "https://app.example/?startapp=dl_token" {
		t.Fatalf("button url = %q, want deep link with startapp param", button.URL)
	}
}
