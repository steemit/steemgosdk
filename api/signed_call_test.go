package api

import (
	"testing"
)

// Test constants
const (
	testURL        = "https://api.steemit.com"
	testAccount    = "testuser"
	testPrivateKey = "5JLw5dgQAx6rhZEgNN5C2ds1V47RweGshynFSWFbaMohsYsBvE8"
	testMethod     = "condenser_api.get_accounts"
)

var testParams = []interface{}{[]string{"testuser"}}

func TestAPI_SignedCall(t *testing.T) {
	api := NewAPI(testURL)

	// Test basic signed call structure (this will fail at network level in tests, but should pass validation)
	_, err := api.SignedCall(testMethod, testParams, testAccount, testPrivateKey)
	
	// We expect a network error in test environment, not a validation error
	if err == nil {
		t.Log("Signed call succeeded (unexpected in test environment)")
	} else {
		// Check that it's not a validation error
		errStr := err.Error()
		if errStr == "signed calls can only be made when using HTTP transport" {
			t.Error("Transport validation failed unexpectedly")
		} else {
			t.Logf("Expected network error in test environment: %v", err)
		}
	}
}

func TestAPI_SignedCallWithInvalidTransport(t *testing.T) {
	// Test with WebSocket URL
	api := NewAPI("wss://api.steemit.com")

	_, err := api.SignedCall(testMethod, testParams, testAccount, testPrivateKey)
	if err == nil {
		t.Error("Expected error for WebSocket transport")
	}

	if err.Error() != "signed calls can only be made when using HTTP transport" {
		t.Errorf("Expected transport error, got: %v", err)
	}
}

func TestAPI_SignedCallWithInvalidPrivateKey(t *testing.T) {
	api := NewAPI(testURL)

	_, err := api.SignedCall(testMethod, testParams, testAccount, "invalid-key")
	if err == nil {
		t.Error("Expected error for invalid private key")
	}

	// Should fail at signing stage, not transport validation
	errStr := err.Error()
	if errStr == "signed calls can only be made when using HTTP transport" {
		t.Error("Should have failed at signing stage, not transport validation")
	}
}

func TestAPI_SignedCallWithResult(t *testing.T) {
	api := NewAPI(testURL)

	var result []map[string]interface{}
	err := api.SignedCallWithResult(testMethod, testParams, testAccount, testPrivateKey, &result)
	
	// We expect a network error in test environment
	if err != nil {
		t.Logf("Expected error in test environment: %v", err)
	}
}

func TestAPI_validateTransportForSignedCall(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "HTTP URL",
			url:     "http://api.steemit.com",
			wantErr: false,
		},
		{
			name:    "HTTPS URL",
			url:     "https://api.steemit.com",
			wantErr: false,
		},
		{
			name:    "WebSocket URL",
			url:     "ws://api.steemit.com",
			wantErr: true,
		},
		{
			name:    "Secure WebSocket URL",
			url:     "wss://api.steemit.com",
			wantErr: true,
		},
		{
			name:    "Invalid URL",
			url:     "ftp://api.steemit.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewAPI(tt.url)
			err := api.validateTransportForSignedCall()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTransportForSignedCall() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAPI_SeqNoIncrement(t *testing.T) {
	api := NewAPI(testURL)
	
	initialSeqNo := api.seqNo
	
	// Make a signed call (will fail at network level, but should increment seqNo)
	api.SignedCall(testMethod, testParams, testAccount, testPrivateKey)
	
	if api.seqNo != initialSeqNo+1 {
		t.Errorf("Expected seqNo to increment from %d to %d, got %d", initialSeqNo, initialSeqNo+1, api.seqNo)
	}
	
	// Make another call
	api.SignedCall(testMethod, testParams, testAccount, testPrivateKey)
	
	if api.seqNo != initialSeqNo+2 {
		t.Errorf("Expected seqNo to increment to %d, got %d", initialSeqNo+2, api.seqNo)
	}
}

// Benchmark tests
func BenchmarkAPI_SignedCall(b *testing.B) {
	api := NewAPI(testURL)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail at network level, but we're measuring the signing overhead
		api.SignedCall(testMethod, testParams, testAccount, testPrivateKey)
	}
}

func BenchmarkAPI_validateTransportForSignedCall(b *testing.B) {
	api := NewAPI(testURL)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		api.validateTransportForSignedCall()
	}
}
