package auth

import (
	"os"
	"testing"
	"time"
)

func TestHtdigestFile(t *testing.T) {
	t.Parallel()
	secrets := HtdigestFileProvider("test.htdigest")
	digest := secrets(nil, "test", "example.com")
	if digest != "aa78524fceb0e50fd8ca96dd818b8cf9" {
		t.Fatal("Incorrect digest for test user:", digest)
	}
	digest = secrets(nil, "test", "example1.com")
	if digest != "" {
		t.Fatal("Got digest for user in non-existant realm:", digest)
	}
	digest = secrets(nil, "test1", "example.com")
	if digest != "" {
		t.Fatal("Got digest for non-existant user:", digest)
	}
}

func TestHtpasswdFile(t *testing.T) {
	t.Parallel()
	secrets := HtpasswdFileProvider("test.htpasswd")
	passwd := secrets(nil, "test", "blah")
	if passwd != "{SHA}qvTGHdzF6KLavt4PO0gs2a6pQ00=" {
		t.Fatal("Incorrect passwd for test user:", passwd)
	}
	passwd = secrets(nil, "nosuchuser", "blah")
	if passwd != "" {
		t.Fatal("Got passwd for non-existant user:", passwd)
	}
}

// TestConcurrent verifies potential race condition in users reading logic
func TestConcurrent(t *testing.T) {
	t.Parallel()
	secrets := HtpasswdFileProvider("test.htpasswd")
	os.Chtimes("test.htpasswd", time.Now(), time.Now())
	go func() {
		secrets(nil, "test", "blah")
	}()
	secrets(nil, "test", "blah")
}
