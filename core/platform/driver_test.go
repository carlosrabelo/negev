package platform

import (
	"testing"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ios lowercase",
			input:    "ios",
			expected: "ios",
		},
		{
			name:     "ios uppercase",
			input:    "IOS",
			expected: "ios",
		},
		{
			name:     "ios mixed case",
			input:    "IoS",
			expected: "ios",
		},
		{
			name:     "dmos lowercase",
			input:    "dmos",
			expected: "dmos",
		},
		{
			name:     "dmos uppercase",
			input:    "DMOS",
			expected: "dmos",
		},
		{
			name:     "dmos mixed case",
			input:    "DmOs",
			expected: "dmos",
		},
		{
			name:     "auto lowercase",
			input:    "auto",
			expected: "auto",
		},
		{
			name:     "auto uppercase",
			input:    "AUTO",
			expected: "auto",
		},
		{
			name:     "auto mixed case",
			input:    "AuTo",
			expected: "auto",
		},
		{
			name:     "with spaces",
			input:    "  ios  ",
			expected: "ios",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		expectError bool
		expectName  string
	}{
		{
			name:        "ios platform",
			platform:    "ios",
			expectError: false,
			expectName:  "ios",
		},
		{
			name:        "dmos platform",
			platform:    "dmos",
			expectError: false,
			expectName:  "dmos",
		},
		{
			name:        "invalid platform",
			platform:    "invalid",
			expectError: true,
		},
		{
			name:        "empty platform",
			platform:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := Get(tt.platform)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for platform %s", tt.platform)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for platform %s: %v", tt.platform, err)
				return
			}

			if driver.Name() != tt.expectName {
				t.Errorf("Expected driver name %s, got %s", tt.expectName, driver.Name())
			}
		})
	}
}

func TestAvailable(t *testing.T) {
	platforms := Available()

	if len(platforms) == 0 {
		t.Error("Available() should return at least one platform")
	}

	expectedPlatforms := []string{"ios", "dmos"}
	for _, expected := range expectedPlatforms {
		found := false
		for _, driver := range platforms {
			if driver.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected platform %s not found in Available() result", expected)
		}
	}
}

func TestDetect(t *testing.T) {
	// Test Detect function with mock repository that doesn't match any platform
	mockRepo := &MockSwitchRepository{}
	driver, err := Detect(mockRepo)

	// Detect should return an error since our mock doesn't simulate a real switch
	if err == nil {
		t.Error("Expected error from Detect with mock repository")
	}

	if driver != nil {
		t.Error("Expected nil driver when detection fails")
	}
}

// MockSwitchRepository implements a minimal repository for testing
type MockSwitchRepository struct {
	connected bool
}

func (m *MockSwitchRepository) Connect() error {
	m.connected = true
	return nil
}

func (m *MockSwitchRepository) Disconnect() {
	m.connected = false
}

func (m *MockSwitchRepository) ExecuteCommand(cmd string) (string, error) {
	return "mock response", nil
}

func (m *MockSwitchRepository) IsConnected() bool {
	return m.connected
}