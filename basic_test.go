package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

var basicSecrets = map[string]string{
	"test":          "{SHA}qvTGHdzF6KLavt4PO0gs2a6pQ00=",
	"test2":         "$apr1$a0j62R97$mYqFkloXH0/UOaUnAiV2b0",
	"test16":        "$apr1$JI4wh3am$AmhephVqLTUyAVpFQeHZC0",
	"test3":         "$2y$05$ih3C91zUBSTFcAh2mQnZYuob0UOZVEf16wl/ukgjDhjvj.xgM1WwS",
	"testsha":       "{SHA}qvTGHdzF6KLavt4PO0gs2a6pQ00=",
	"testmd5":       "$apr1$0.KbAJur$4G9MiqUjDLCuihkMfmg6e1",
	"testmd5broken": "$apr10.KbAJur$4G9MiqUjDLCuihkMfmg6e1",
}

type credentials struct {
	username, password string
}

var basicCredentials = []credentials{
	{"test", "hello"},
	{"test2", "hello2"},
	{"test3", "hello3"},
	{"test16", "topsecret"},
}

func basicProvider(r *http.Request, user, realm string) string {
	return basicSecrets[user]
}

func TestBasicCheckAuthFailsOnBadHeaders(t *testing.T) {
	t.Parallel()
	a := &BasicAuth{Realm: "example.com", Secrets: basicProvider}
	for _, auth := range []string{
		"",
		"Digest blabla ololo",
		"Basic !@#",
	} {
		r, err := http.NewRequest("GET", "http://example.com", nil)
		if err != nil {
			t.Fatal(err)
		}
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		if a.CheckAuth(r) != "" {
			t.Errorf("CheckAuth returned a username for Authorization header %q", r.Header.Get("Authorization"))
		}
	}
}

func TestBasicCheckAuth(t *testing.T) {
	t.Parallel()
	a := &BasicAuth{Realm: "example.com", Secrets: basicProvider}
	for _, creds := range basicCredentials {
		r, err := http.NewRequest("GET", "http://example.com", nil)
		if err != nil {
			t.Fatal(err)
		}
		r.SetBasicAuth(creds.username, creds.password)
		if a.CheckAuth(r) != creds.username {
			t.Fatalf("CheckAuth failed for user '%s'", creds.username)
		}
	}
}

func TestBasicAuthContext(t *testing.T) {
	t.Parallel()
	a := &BasicAuth{Realm: "example.com", Secrets: basicProvider}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := a.NewContext(r.Context(), r)
		authInfo := FromContext(ctx)
		authInfo.UpdateHeaders(w.Header())
		if authInfo == nil || !authInfo.Authenticated {
			http.Error(w, "error", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, authInfo.Username)
	}))
	defer ts.Close()
	for _, tt := range []struct {
		username, password string
		want               int
	}{
		{"", "", http.StatusUnauthorized},
		{"test", "hello", http.StatusOK},
		{"testmd5", "hello", http.StatusOK},
		{"testmd5broken", "hello", http.StatusUnauthorized},
		{"testmd5", "invalid", http.StatusUnauthorized},
	} {
		r, err := http.NewRequest("GET", ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.SetBasicAuth(tt.username, tt.password)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		if resp.StatusCode != tt.want {
			t.Errorf("user %q, password %q: got status %d, want %d", tt.username, tt.password, resp.StatusCode, tt.want)
		}
	}
}

func TestBasicAuthWrap(t *testing.T) {
	t.Parallel()
	a := NewBasicAuthenticator("example.com", basicProvider)
	ts := httptest.NewServer(http.HandlerFunc(a.Wrap(func(w http.ResponseWriter, r *AuthenticatedRequest) {
		if r.Username == "" {
			http.Error(w, "error", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, r.Username)
	})))
	defer ts.Close()
	for _, tt := range []struct {
		username, password string
		want               int
	}{
		{"", "", http.StatusUnauthorized},
		{"testsha", "invalid", http.StatusUnauthorized},
		{"testsha", "hello", http.StatusOK},
	} {
		r, err := http.NewRequest("GET", ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.SetBasicAuth(tt.username, tt.password)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		if resp.StatusCode != tt.want {
			t.Errorf("user %q, password %q: got status %d, want %d", tt.username, tt.password, resp.StatusCode, tt.want)
		}
	}
}

func TestCheckSecret(t *testing.T) {
	t.Parallel()
	// alike cases are tested in users_test.go
	data := [][]string{
		// generated by htpasswd=2.4.18 and openssl=1.0.2g
		{"htpasswd-md5", "$apr1$FVVioVP7$ZdIWPG1p4E/ErujO7kA2n0"},
		{"openssl-apr1", "$apr1$peiE49Vv$lo.z77Z.6.a.Lm7GMjzQh0"},
		{"openssl-md5", "$1$mvmz31IB$U9KpHBLegga2doA0e3s3N0"},
		{"htpasswd-sha", "{SHA}vFznddje0Ht4+pmO0FaxwrUKN/M="},
		{"htpasswd-bcrypt", "$2y$10$Q6GeMFPd0dAxhQULPDdAn.DFy6NDmLaU0A7e2XoJz7PFYAEADFKbC"},
		// common bcrypt test vectors
		{"", "$2a$06$DCq7YPn5Rq63x1Lad4cll.TV4S6ytwfsfvkgY8jIucDrjc8deX1s."},
		{"", "$2a$08$HqWuK6/Ng6sg9gQzbLrgb.Tl.ZHfXLhvt/SgVyWhQqgqcZ7ZuUtye"},
		{"", "$2a$10$k1wbIrmNyFAPwPVPSVa/zecw2BCEnBwVS2GbrmgzxFUOqW9dk4TCW"},
		{"", "$2a$12$k42ZFHFWqBp3vWli.nIn8uYyIkbvYRvodzbfbK18SSsY.CsIQPlxO"},
		{"a", "$2a$06$m0CrhHm10qJ3lXRY.5zDGO3rS2KdeeWLuGmsfGlMfOxih58VYVfxe"},
		{"a", "$2a$08$cfcvVd2aQ8CMvoMpP2EBfeodLEkkFJ9umNEfPD18.hUF62qqlC/V."},
		{"a", "$2a$10$k87L/MF28Q673VKh8/cPi.SUl7MU/rWuSiIDDFayrKk/1tBsSQu4u"},
		{"a", "$2a$12$8NJH3LsPrANStV6XtBakCez0cKHXVxmvxIlcz785vxAIZrihHZpeS"},
		{"abc", "$2a$06$If6bvum7DFjUnE9p2uDeDu0YHzrHM6tf.iqN8.yx.jNN1ILEf7h0i"},
		{"abc", "$2a$08$Ro0CUfOqk6cXEKf3dyaM7OhSCvnwM9s4wIX9JeLapehKK5YdLxKcm"},
		{"abc", "$2a$10$WvvTPHKwdBJ3uk0Z37EMR.hLA2W6N9AEBhEgrAOljy2Ae5MtaSIUi"},
		{"abc", "$2a$12$EXRkfkdmXn2gzds2SSitu.MW9.gAVqa9eLS1//RYtYCmB1eLHg.9q"},
		{"abcdefghijklmnopqrstuvwxyz", "$2a$06$.rCVZVOThsIa97pEDOxvGuRRgzG64bvtJ0938xuqzv18d3ZpQhstC"},
		{"abcdefghijklmnopqrstuvwxyz", "$2a$08$aTsUwsyowQuzRrDqFflhgekJ8d9/7Z3GV3UcgvzQW3J5zMyrTvlz."},
		{"abcdefghijklmnopqrstuvwxyz", "$2a$10$fVH8e28OQRj9tqiDXs1e1uxpsjN0c7II7YPKXua2NAKYvM6iQk7dq"},
		{"abcdefghijklmnopqrstuvwxyz", "$2a$12$D4G5f18o7aMMfwasBL7GpuQWuP3pkrZrOAnqP.bmezbMng.QwJ/pG"},
		{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$06$fPIsBO8qRqkjj273rfaOI.HtSV9jLDpTbZn782DC6/t7qT67P6FfO"},
		{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$08$Eq2r4G/76Wv39MzSX262huzPz612MZiYHVUJe/OcOql2jo4.9UxTW"},
		{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$10$LgfYWkbzEvQ4JakH7rOvHe0y8pHKF9OaFgwUZ2q7W2FFZmZzJYlfS"},
		{"~!@#$%^&*()      ~!@#$%^&*()PNBFRD", "$2a$12$WApznUOJfkEGSmYRfnkrPOr466oFDCaj4b6HY3EXGvfxm43seyhgC"},
		// unicode test vector
		{"ππππππππ", "$2a$10$.TtQJ4Jr6isd4Hp.mVfZeuh6Gws4rOQ/vdBczhDx.19NFK0Y84Dle"},
		// TODO: add test vectors for `$2b` and `$2x`
	}
	for i, tc := range data {
		t.Run(fmt.Sprintf("Vector%d", i), func(t *testing.T) {
			t.Parallel()
			password, secret := tc[0], tc[1]
			if !CheckSecret(password, secret) {
				t.Error("CheckSecret returned false, want true")
			}
			if CheckSecret(password+"x", secret) {
				t.Error("CheckSecret returned true for invalid password, want false")
			}
			secret = secret[0:len(secret)-1] + "x"
			if CheckSecret(password, secret) {
				t.Error("CheckSecret returned true for invalid secret, want false")
			}
		})
	}
}
