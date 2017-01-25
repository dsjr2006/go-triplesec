package triplesec

import (
	"fmt"
)

// CorruptionError indicates that the encrypted item is corrupted
type CorruptionError struct {
	msg string
}

func (e CorruptionError) Error() string {
	return "Triplesec corruption: " + e.msg
}

// VersionError indicates a version mismatch or unsuppported version
type VersionError struct {
	v uint32
}

func (e VersionError) Error() string {
	return fmt.Sprintf("Unknown version: %v", e.v)
}

// BadPassphraseError indicates an incorrect passphrase or failed MAC
type BadPassphraseError struct{}

func (e BadPassphraseError) Error() string {
	return "Bad passphrase (or inflight message tampering)"
}
