package auth

import "testing"

func TestHashVerifiesOnlySamePassword(t *testing.T) {
	hash, err := Hash("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(hash, "correct-horse") {
		t.Fatal("same password rejected")
	}
	if Verify(hash, "wrong-password") {
		t.Fatal("wrong password accepted")
	}
	other, err := Hash("correct-horse")
	if err != nil || other == hash {
		t.Fatal("salt was not random")
	}
}
