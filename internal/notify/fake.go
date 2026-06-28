package notify

import "sync"

// FakeMailer captures messages instead of sending them. Safe for concurrent use.
type FakeMailer struct {
	mu   sync.Mutex
	Sent []Message
	// Err, if set, is returned by Send (to test failure paths).
	Err error
}

// NewFakeMailer returns an empty fake mailer.
func NewFakeMailer() *FakeMailer { return &FakeMailer{} }

// Send records msg (or returns the configured error).
func (f *FakeMailer) Send(msg Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.Sent = append(f.Sent, msg)
	return nil
}

// Messages returns a copy of the captured messages.
func (f *FakeMailer) Messages() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Message, len(f.Sent))
	copy(out, f.Sent)
	return out
}

// Count returns how many messages have been captured.
func (f *FakeMailer) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Sent)
}
