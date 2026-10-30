package session

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	plain := []byte("hello tg-dlp")
	enc, err := EncryptData(plain, "secret")
	if err != nil {
		t.Fatal(err)
	}
	dec, err := DecryptData(enc, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != string(plain) {
		t.Errorf("got %q, want %q", dec, plain)
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	enc, _ := EncryptData([]byte("data"), "correct")
	if _, err := DecryptData(enc, "wrong"); err == nil {
		t.Error("expected error with wrong password")
	}
}

func TestDecryptCorrupted(t *testing.T) {
	if _, err := DecryptData([]byte("short"), "pw"); err == nil {
		t.Error("expected error with corrupted data")
	}
}