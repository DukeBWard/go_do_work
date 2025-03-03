package queue

import (
	"errors"
	"testing"
	"time"
)

// tests the default retry policy
func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", policy.MaxRetries)
	}

	if policy.BaseDelay != 1*time.Second {
		t.Errorf("Expected BaseDelay to be 1s, got %v", policy.BaseDelay)
	}

	if policy.MaxDelay != 1*time.Minute {
		t.Errorf("Expected MaxDelay to be 1m, got %v", policy.MaxDelay)
	}

	if policy.Strategy != RetryExponentialJitter {
		t.Errorf("Expected Strategy to be RetryExponentialJitter, got %v", policy.Strategy)
	}

	// Test the ShouldRetry function
	if !policy.ShouldRetry(errors.New("test error")) {
		t.Error("Expected ShouldRetry to return true for non-nil error")
	}

	if policy.ShouldRetry(nil) {
		t.Error("Expected ShouldRetry to return false for nil error")
	}
}

// tests retry delay calculation for different strategies
func TestCalculateRetryDelay(t *testing.T) {
	// Test immediate retry
	immediatePolicy := RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		Strategy:   RetryImmediate,
	}

	delay := CalculateRetryDelay(immediatePolicy, 1)
	if delay != 0 {
		t.Errorf("Expected immediate retry delay to be 0, got %v", delay)
	}

	// Test fixed retry
	fixedPolicy := RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  2 * time.Second,
		MaxDelay:   10 * time.Second,
		Strategy:   RetryFixed,
	}

	delay = CalculateRetryDelay(fixedPolicy, 1)
	if delay != 2*time.Second {
		t.Errorf("Expected fixed retry delay to be 2s, got %v", delay)
	}

	// Test exponential retry
	expPolicy := RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		Strategy:   RetryExponential,
	}

	delay = CalculateRetryDelay(expPolicy, 2)
	expectedDelay := 2 * time.Second // 1s * 2^(2-1) = 1s * 2^1 = 2s
	if delay != expectedDelay {
		t.Errorf("Expected exponential retry delay to be %v, got %v", expectedDelay, delay)
	}

	// Test max delay cap
	delay = CalculateRetryDelay(expPolicy, 5)
	if delay > expPolicy.MaxDelay {
		t.Errorf("Expected delay to be capped at %v, got %v", expPolicy.MaxDelay, delay)
	}
}

// tests the shouldretryerror function
func TestShouldRetryError(t *testing.T) {
	policy := RetryPolicy{
		MaxRetries: 3,
		ShouldRetry: func(err error) bool {
			return err != nil && err.Error() == "retry me"
		},
	}

	// Test error that should be retried
	if !ShouldRetryError(policy, errors.New("retry me"), 1) {
		t.Error("Expected ShouldRetryError to return true for 'retry me' error")
	}

	// Test error that should not be retried
	if ShouldRetryError(policy, errors.New("don't retry me"), 1) {
		t.Error("Expected ShouldRetryError to return false for 'don't retry me' error")
	}

	// Test max retries exceeded
	if ShouldRetryError(policy, errors.New("retry me"), 4) {
		t.Error("Expected ShouldRetryError to return false when max retries exceeded")
	}
}
