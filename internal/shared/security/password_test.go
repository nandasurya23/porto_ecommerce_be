package security

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := CheckPassword("password123", hash); err != nil {
		t.Fatalf("CheckPassword failed: %v", err)
	}
	if err := CheckPassword("wrong-password", hash); err == nil {
		t.Fatal("CheckPassword should fail for wrong password")
	}
}
