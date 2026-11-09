package signal

// notesProjectError preserves the established display text for every signal
// verb while retaining the wrapped resolver cause for classification.
type notesProjectError struct {
	message string
	cause   error
}

func (e notesProjectError) Error() string { return e.message }
func (e notesProjectError) Unwrap() error { return e.cause }
