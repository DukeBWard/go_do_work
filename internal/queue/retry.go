package queue

import (
	"math"
	"math/rand/v2"
	"time"
)

type RetryStrategy int

// this uses implicit types
// because we mark the first one explicity as retry strategy, Go knows the others within the const block
// are most likely also RetryStrategy = iota
// could also explicity list each one if you wanted to
// this means that RetryImmediate actually = 1, RetryFixed = 2, ...
const (
	// retries immediately no delay
	RetryImmediate RetryStrategy = iota
	// fixed delay between retries
	RetryFixed
	// exponential backoff between retries
	RetryExponential
	// random jitter to exponential backoff
	RetryExponentialJitter
)

type RetryPolicy struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Strategy    RetryStrategy
	ShouldRetry func(error) bool
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   1 * time.Minute,
		Strategy:   RetryExponentialJitter,
		ShouldRetry: func(err error) bool {
			// by default retry all errors
			return err != nil
		},
	}
}

// withmaxretries creates an option that sets the maximum retry attempts
func WithMaxRetries(maxRetries int) Option {
	return func(o *Options) {
		if maxRetries >= 0 {
			o.retryPolicy.MaxRetries = maxRetries
		}
	}
}

// withbasedelay creates an option that sets the base delay for retries
func WithBaseDelay(delay time.Duration) Option {
	return func(o *Options) {
		if delay > 0 {
			o.retryPolicy.BaseDelay = delay
		}
	}
}

// withmaxdelay creates an option that sets the maximum delay for retries
func WithMaxDelay(delay time.Duration) Option {
	return func(o *Options) {
		if delay > 0 {
			o.retryPolicy.MaxDelay = delay
		}
	}
}

// withretrystrategy creates an option that sets the retry strategy
func WithRetryStrategy(strategy RetryStrategy) Option {
	return func(o *Options) {
		o.retryPolicy.Strategy = strategy
	}
}

// withcustomretrycheck creates an option that sets a custom function to determine
// if a specific error should trigger a retry
func WithCustomRetryCheck(shouldRetry func(error) bool) Option {
	return func(o *Options) {
		if shouldRetry != nil {
			o.retryPolicy.ShouldRetry = shouldRetry
		}
	}
}

// compute the delay before the next retry attempt
func CalculateRetryDelay(policy RetryPolicy, attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	switch policy.Strategy {
	case RetryImmediate:
		return 0

	case RetryFixed:
		return policy.BaseDelay

	case RetryExponential:
		// calculate delay with exponential backoff: baseDelay * 2^attempt
		delay := float64(policy.BaseDelay) * math.Pow(2, float64(attempt-1))

		// cap at max delay
		if delay > float64(policy.MaxDelay) {
			delay = float64(policy.MaxDelay)
		}

		return time.Duration(delay)

	case RetryExponentialJitter:
		// calculate delay with exponential backoff
		delay := float64(policy.BaseDelay) * math.Pow(2, float64(attempt-1))

		// cap at max delay
		if delay > float64(policy.MaxDelay) {
			delay = float64(policy.MaxDelay)
		}

		// add jitter: random value between 0.5*delay and 1.5*delay
		jitterFactor := 0.5 + rand.Float64()
		delay = delay * jitterFactor

		return time.Duration(delay)

	default:
		// default to fixed delay if strategy is unknown
		return policy.BaseDelay
	}
}

// shouldretryerror determines if the given error should be retried based on the policy
func ShouldRetryError(policy RetryPolicy, err error, attempts int) bool {
	// check if maximum retry attempts have been exceeded
	if attempts > policy.MaxRetries {
		return false
	}

	// use the policy's custom retry checker if provided
	if policy.ShouldRetry != nil {
		return policy.ShouldRetry(err)
	}

	// default behavior: retry if there's an error
	return err != nil
}

// isretryableerror is a utility function that determines if a specific error
// is considered retryable (can be used with withcustomretrycheck)
func IsRetryableError(err error) bool {
	// this is a placeholder. in a real implementation, you would:
	// 1. check for specific error types that should be retried
	// 2. check for specific error types that should NOT be retried

	// example implementation:
	if err == nil {
		return false
	}

	// example: don't retry permanent errors (you'd define this somewhere)
	// if errors.Is(err, ErrPermanent) {
	//     return false
	// }

	// by default, consider all other errors as retryable
	return true
}
