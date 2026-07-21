package worker

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewProcessError(t *testing.T) {
	tests := []struct {
		name      string
		retryable bool
	}{
		{name: "retryable", retryable: true},
		{name: "non-retryable", retryable: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewProcessError(tc.retryable, "update %s/%s: %s", "default", "my-dep", "boom")

			if got, want := err.Error(), "update default/my-dep: boom"; got != want {
				t.Errorf("Error() = %q, want %q", got, want)
			}
			if err.Retryable() != tc.retryable {
				t.Errorf("Retryable() = %v, want %v", err.Retryable(), tc.retryable)
			}
		})
	}
}

func TestProcessError_Is(t *testing.T) {
	retryableA := NewProcessError(true, "error A")
	retryableB := NewProcessError(true, "error B")
	permanentA := NewProcessError(false, "error C")

	// Is matches solely on the retryable flag, regardless of message.
	if !errors.Is(retryableA, retryableB) {
		t.Error("expected two retryable ProcessErrors with different messages to match via errors.Is")
	}
	if errors.Is(retryableA, permanentA) {
		t.Error("expected retryable and non-retryable ProcessErrors not to match via errors.Is")
	}

	// Sentinels behave as the retryable-flag class representatives.
	if !errors.Is(retryableA, RetryableError) {
		t.Error("expected retryable ProcessError to match RetryableError sentinel")
	}
	if errors.Is(retryableA, UnretryableError) {
		t.Error("expected retryable ProcessError not to match UnretryableError sentinel")
	}
	if !errors.Is(permanentA, UnretryableError) {
		t.Error("expected non-retryable ProcessError to match UnretryableError sentinel")
	}
	if errors.Is(permanentA, RetryableError) {
		t.Error("expected non-retryable ProcessError not to match RetryableError sentinel")
	}
}

func TestProcessError_Is_NonProcessError(t *testing.T) {
	err := NewProcessError(true, "boom")
	other := fmt.Errorf("some other error")

	if errors.Is(err, other) {
		t.Error("expected ProcessError not to match a non-ProcessError via errors.Is")
	}
	if errors.Is(other, RetryableError) {
		t.Error("expected a plain error not to match the RetryableError sentinel")
	}
}

func TestErrorsIs_JoinedErrors_RetryableIfAnyRetryable(t *testing.T) {
	// errors.Join + errors.Is: a batch is treated as retryable if ANY sub-error is retryable,
	// which is exactly the semantics CRDWorker.process relies on to decide whether to requeue.
	joined := errors.Join(
		NewProcessError(false, "permanent failure on item 1"),
		NewProcessError(true, "transient failure on item 2"),
	)

	if !errors.Is(joined, RetryableError) {
		t.Error("expected joined error containing a retryable sub-error to match RetryableError")
	}

	allPermanent := errors.Join(
		NewProcessError(false, "permanent failure on item 1"),
		NewProcessError(false, "permanent failure on item 2"),
	)
	if errors.Is(allPermanent, RetryableError) {
		t.Error("expected joined error with only non-retryable sub-errors not to match RetryableError")
	}
	if !errors.Is(allPermanent, UnretryableError) {
		t.Error("expected joined error with only non-retryable sub-errors to match UnretryableError")
	}
}
