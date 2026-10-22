package auth

import (
	"errors"
	"testing"

	"golang.org/x/oauth2"
)

func TestRejectedByProvider(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&oauth2.RetrieveError{ErrorCode: "invalid_grant"}, true},
		{&oauth2.RetrieveError{ErrorCode: "invalid_client"}, true},
		{&oauth2.RetrieveError{ErrorCode: "temporarily_unavailable"}, false},
		{errors.New("dial tcp: no such host"), false},
	}
	for _, c := range cases {
		if got := rejectedByProvider(c.err); got != c.want {
			t.Errorf("%v: got %v want %v", c.err, got, c.want)
		}
	}
}

func TestConfigEnabled(t *testing.T) {
	if (Config{}).Enabled() {
		t.Error("empty config must be disabled")
	}
	if !(Config{Issuer: "https://x", ClientID: "id"}).Enabled() {
		t.Error("issuer + client id must be enabled")
	}
}
