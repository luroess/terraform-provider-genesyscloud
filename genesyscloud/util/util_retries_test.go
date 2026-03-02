package util

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/mypurecloud/platform-client-sdk-go/v179/platformclientv2"
)

func TestUnitGetRetryAfterDelay_IntegerFormat(t *testing.T) {
	// Test with a static integer value between 5 and 60 seconds
	seconds := 25
	expectedDelay := time.Duration(seconds) * time.Second

	// Create mock APIResponse with integer Retry-After header
	apiResponse := &platformclientv2.APIResponse{
		Response: &http.Response{
			Header: make(http.Header),
		},
	}
	apiResponse.Response.Header.Set("Retry-After", strconv.Itoa(seconds))

	delay, ok := GetRetryAfterDelay(apiResponse)
	if !ok {
		t.Errorf("Expected GetRetryAfterDelay to return true for integer value %d, got false", seconds)
	}
	if delay != expectedDelay {
		t.Errorf("Expected delay %v for integer value %d, got %v", expectedDelay, seconds, delay)
	}
}

func TestUnitGetRetryAfterDelay_NoHeader(t *testing.T) {
	// Test with no Retry-After header
	apiResponse := &platformclientv2.APIResponse{
		Response: &http.Response{
			Header: make(http.Header),
		},
	}

	delay, ok := GetRetryAfterDelay(apiResponse)
	if ok {
		t.Errorf("Expected GetRetryAfterDelay to return false when header is missing, got true")
	}
	if delay != 0 {
		t.Errorf("Expected delay 0 when header is missing, got %v", delay)
	}
}

func TestUnitGetRetryAfterDelay_EmptyHeader(t *testing.T) {
	// Test with empty Retry-After header
	apiResponse := &platformclientv2.APIResponse{
		Response: &http.Response{
			Header: make(http.Header),
		},
	}
	apiResponse.Response.Header.Set("Retry-After", "")

	delay, ok := GetRetryAfterDelay(apiResponse)
	if ok {
		t.Errorf("Expected GetRetryAfterDelay to return false when header is empty, got true")
	}
	if delay != 0 {
		t.Errorf("Expected delay 0 when header is empty, got %v", delay)
	}
}

func TestUnitGetRetryAfterDelay_NilResponse(t *testing.T) {
	// Test with nil APIResponse
	delay, ok := GetRetryAfterDelay(nil)
	if ok {
		t.Errorf("Expected GetRetryAfterDelay to return false when APIResponse is nil, got true")
	}
	if delay != 0 {
		t.Errorf("Expected delay 0 when APIResponse is nil, got %v", delay)
	}
}

func TestUnitGetRetryAfterDelay_NilResponseResponse(t *testing.T) {
	// Test with nil Response field
	apiResponse := &platformclientv2.APIResponse{
		Response: nil,
	}

	delay, ok := GetRetryAfterDelay(apiResponse)
	if ok {
		t.Errorf("Expected GetRetryAfterDelay to return false when Response is nil, got true")
	}
	if delay != 0 {
		t.Errorf("Expected delay 0 when Response is nil, got %v", delay)
	}
}

func TestUnitGetRetryAfterDelay_InvalidFormat(t *testing.T) {
	// Test with invalid format
	apiResponse := &platformclientv2.APIResponse{
		Response: &http.Response{
			Header: make(http.Header),
		},
	}
	apiResponse.Response.Header.Set("Retry-After", "invalid-format")

	delay, ok := GetRetryAfterDelay(apiResponse)
	if ok {
		t.Errorf("Expected GetRetryAfterDelay to return false for invalid format, got true")
	}
	if delay != 0 {
		t.Errorf("Expected delay 0 for invalid format, got %v", delay)
	}
}

func TestUnitGetRetryAfterDelay_ZeroSeconds(t *testing.T) {
	// Test with zero seconds (should return false)
	apiResponse := &platformclientv2.APIResponse{
		Response: &http.Response{
			Header: make(http.Header),
		},
	}
	apiResponse.Response.Header.Set("Retry-After", "0")

	delay, ok := GetRetryAfterDelay(apiResponse)
	if ok {
		t.Errorf("Expected GetRetryAfterDelay to return false for zero seconds, got true")
	}
	if delay != 0 {
		t.Errorf("Expected delay 0 for zero seconds, got %v", delay)
	}
}

func TestUnitGetRetryAfterDelay_NegativeSeconds(t *testing.T) {
	// Test with negative seconds (should return false)
	apiResponse := &platformclientv2.APIResponse{
		Response: &http.Response{
			Header: make(http.Header),
		},
	}
	apiResponse.Response.Header.Set("Retry-After", "-5")

	delay, ok := GetRetryAfterDelay(apiResponse)
	// Negative seconds should fail to parse as integer, so it should return false
	if ok {
		t.Errorf("Expected GetRetryAfterDelay to return false for negative seconds, got true")
	}
	if delay != 0 {
		t.Errorf("Expected delay 0 for negative seconds, got %v", delay)
	}
}

// TestUnitRetryWhenExponentialBackoff verifies that RetryWhen uses true exponential backoff
// (delay doubles each retry: 500ms * 2^i) rather than linear backoff ((i+1)*500ms).
func TestUnitRetryWhenExponentialBackoff(t *testing.T) {
	prevMax := SetMaxRetriesForTests(4)
	defer SetMaxRetriesForTests(prevMax)

	var delays []time.Duration
	callCount := 0

	alwaysRetry := func(resp *platformclientv2.APIResponse, additionalCodes ...int) bool {
		return true
	}

	var prevCallTime time.Time
	callSdk := func() (*platformclientv2.APIResponse, diag.Diagnostics) {
		now := time.Now()
		if callCount > 0 {
			delays = append(delays, now.Sub(prevCallTime))
		}
		prevCallTime = now
		callCount++
		resp := &platformclientv2.APIResponse{
			Response: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
			},
		}
		return resp, diag.Diagnostics{diag.Diagnostic{
			Severity: diag.Error,
			Summary:  fmt.Sprintf("retry %d", callCount),
		}}
	}

	_ = RetryWhen(alwaysRetry, callSdk)

	// We set maxRetries=4, so callSdk is called 4 times.
	// delays[0] = delay after call 1 (i=0): 500ms * 2^0 = 500ms
	// delays[1] = delay after call 2 (i=1): 500ms * 2^1 = 1000ms
	// delays[2] = delay after call 3 (i=2): 500ms * 2^2 = 2000ms
	if len(delays) != 3 {
		t.Fatalf("Expected 3 measured delays (4 retries), got %d", len(delays))
	}

	// Verify each delay is approximately double the previous (exponential growth).
	// Allow ±20% tolerance for timing variance.
	for i, measuredDelay := range delays {
		expected := time.Duration(1<<uint(i)) * 500 * time.Millisecond
		ratio := float64(measuredDelay) / float64(expected)
		if ratio < 0.8 || ratio > 1.2 {
			t.Errorf("Delay[%d]: expected ~%v (exponential backoff 500ms*2^%d), got %v (ratio=%.2f)", i, expected, i, measuredDelay, ratio)
		}
	}

	// Also verify exponential growth: each delay should be roughly double the previous.
	// delays[1] / delays[0] ≈ 2, delays[2] / delays[1] ≈ 2
	for i := 1; i < len(delays); i++ {
		ratio := float64(delays[i]) / float64(delays[i-1])
		if ratio < 1.6 || ratio > 2.4 {
			t.Errorf("Expected exponential doubling between delay[%d] (%v) and delay[%d] (%v), got ratio=%.2f", i-1, delays[i-1], i, delays[i], ratio)
		}
	}
}
