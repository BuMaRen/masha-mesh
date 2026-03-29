package storage

import (
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to create a test EndpointSlice
func createTestEndpointSlice(name, serviceName, resourceVersion string) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: resourceVersion,
			Labels: map[string]string{
				"kubernetes.io/service-name": serviceName,
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
			},
		},
	}
}

func TestEndpointSlice_OnAdded(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		initialES      map[string]*discoveryv1.EndpointSlice
		inputES        *discoveryv1.EndpointSlice
		expectedCount  int
		expectedExists bool
	}{
		{
			name:           "add new endpointslice",
			serviceName:    "test-service",
			initialES:      make(map[string]*discoveryv1.EndpointSlice),
			inputES:        createTestEndpointSlice("es-1", "test-service", "1"),
			expectedCount:  1,
			expectedExists: true,
		},
		{
			name:           "add endpointslice with mismatched service name",
			serviceName:    "test-service",
			initialES:      make(map[string]*discoveryv1.EndpointSlice),
			inputES:        createTestEndpointSlice("es-1", "other-service", "1"),
			expectedCount:  0,
			expectedExists: false,
		},
		{
			name:        "add duplicate endpointslice with correct version increment",
			serviceName: "test-service",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
			},
			inputES:        createTestEndpointSlice("es-1", "test-service", "2"),
			expectedCount:  1,
			expectedExists: true,
		},
		{
			name:        "add duplicate endpointslice with incorrect version",
			serviceName: "test-service",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
			},
			inputES:        createTestEndpointSlice("es-1", "test-service", "5"),
			expectedCount:  1,
			expectedExists: true, // old version should remain
		},
		{
			name:           "add to empty service name",
			serviceName:    "",
			initialES:      make(map[string]*discoveryv1.EndpointSlice),
			inputES:        createTestEndpointSlice("es-1", "test-service", "1"),
			expectedCount:  0,
			expectedExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EndpointSlice{
				serviceName: tt.serviceName,
				esNameToEs:  tt.initialES,
			}

			e.OnAdded(tt.inputES)

			if len(e.esNameToEs) != tt.expectedCount {
				t.Errorf("expected %d endpointslices, got %d", tt.expectedCount, len(e.esNameToEs))
			}

			_, exists := e.esNameToEs[tt.inputES.Name]
			if exists != tt.expectedExists {
				t.Errorf("expected exists=%v, got exists=%v", tt.expectedExists, exists)
			}

			// Verify version when it should exist
			if tt.expectedExists && tt.expectedCount > 0 {
				actualES := e.esNameToEs[tt.inputES.Name]
				// For incorrect version increment, old version should remain
				if tt.name == "add duplicate endpointslice with incorrect version" {
					if actualES.ResourceVersion != "1" {
						t.Errorf("expected version to remain '1', got '%s'", actualES.ResourceVersion)
					}
				}
			}
		})
	}
}

func TestEndpointSlice_OnUpdate(t *testing.T) {
	tests := []struct {
		name              string
		serviceName       string
		initialES         map[string]*discoveryv1.EndpointSlice
		oldES             *discoveryv1.EndpointSlice
		newES             *discoveryv1.EndpointSlice
		expectedCount     int
		expectedVersion   string
		shouldExistOldKey bool
		shouldExistNewKey bool
	}{
		{
			name:        "update existing endpointslice with correct version",
			serviceName: "test-service",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
			},
			oldES:             createTestEndpointSlice("es-1", "test-service", "1"),
			newES:             createTestEndpointSlice("es-1", "test-service", "2"),
			expectedCount:     1,
			expectedVersion:   "2",
			shouldExistOldKey: true,
			shouldExistNewKey: true,
		},
		{
			name:              "update non-existent endpointslice",
			serviceName:       "test-service",
			initialES:         make(map[string]*discoveryv1.EndpointSlice),
			oldES:             createTestEndpointSlice("es-1", "test-service", "1"),
			newES:             createTestEndpointSlice("es-1", "test-service", "2"),
			expectedCount:     0,
			shouldExistOldKey: false,
			shouldExistNewKey: false,
		},
		{
			name:        "update with incorrect version increment",
			serviceName: "test-service",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
			},
			oldES:             createTestEndpointSlice("es-1", "test-service", "1"),
			newES:             createTestEndpointSlice("es-1", "test-service", "5"),
			expectedCount:     1,
			expectedVersion:   "1", // should remain unchanged
			shouldExistOldKey: true,
			shouldExistNewKey: true,
		},
		{
			name:        "update with mismatched service name",
			serviceName: "test-service",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
			},
			oldES:             createTestEndpointSlice("es-1", "other-service", "1"),
			newES:             createTestEndpointSlice("es-1", "other-service", "2"),
			expectedCount:     1,
			expectedVersion:   "1", // should remain unchanged
			shouldExistOldKey: true,
			shouldExistNewKey: true,
		},
		{
			name:        "update with name change",
			serviceName: "test-service",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
			},
			oldES:             createTestEndpointSlice("es-1", "test-service", "1"),
			newES:             createTestEndpointSlice("es-2", "test-service", "2"),
			expectedCount:     1,
			expectedVersion:   "2",
			shouldExistOldKey: false,
			shouldExistNewKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EndpointSlice{
				serviceName: tt.serviceName,
				esNameToEs:  tt.initialES,
			}

			e.OnUpdate(tt.oldES, tt.newES)

			if len(e.esNameToEs) != tt.expectedCount {
				t.Errorf("expected %d endpointslices, got %d", tt.expectedCount, len(e.esNameToEs))
			}

			_, oldExists := e.esNameToEs[tt.oldES.Name]
			if oldExists != tt.shouldExistOldKey {
				t.Errorf("expected old key exists=%v, got exists=%v", tt.shouldExistOldKey, oldExists)
			}

			_, newExists := e.esNameToEs[tt.newES.Name]
			if newExists != tt.shouldExistNewKey {
				t.Errorf("expected new key exists=%v, got exists=%v", tt.shouldExistNewKey, newExists)
			}

			if tt.expectedVersion != "" && tt.shouldExistNewKey {
				actualES := e.esNameToEs[tt.newES.Name]
				if actualES.ResourceVersion != tt.expectedVersion {
					t.Errorf("expected version '%s', got '%s'", tt.expectedVersion, actualES.ResourceVersion)
				}
			}
		})
	}
}

func TestEndpointSlice_OnDelete(t *testing.T) {
	tests := []struct {
		name          string
		serviceName   string
		initialES     map[string]*discoveryv1.EndpointSlice
		deleteES      *discoveryv1.EndpointSlice
		expectedCount int
		shouldExist   bool
	}{
		{
			name:        "delete existing endpointslice",
			serviceName: "test-service",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
				"es-2": createTestEndpointSlice("es-2", "test-service", "1"),
			},
			deleteES:      createTestEndpointSlice("es-1", "test-service", "1"),
			expectedCount: 1,
			shouldExist:   false,
		},
		{
			name:          "delete non-existent endpointslice",
			serviceName:   "test-service",
			initialES:     make(map[string]*discoveryv1.EndpointSlice),
			deleteES:      createTestEndpointSlice("es-1", "test-service", "1"),
			expectedCount: 0,
			shouldExist:   false,
		},
		{
			name:        "delete with mismatched service name",
			serviceName: "test-service",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
			},
			deleteES:      createTestEndpointSlice("es-1", "other-service", "1"),
			expectedCount: 1,
			shouldExist:   true,
		},
		{
			name:        "delete from empty service name",
			serviceName: "",
			initialES: map[string]*discoveryv1.EndpointSlice{
				"es-1": createTestEndpointSlice("es-1", "test-service", "1"),
			},
			deleteES:      createTestEndpointSlice("es-1", "test-service", "1"),
			expectedCount: 1,
			shouldExist:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EndpointSlice{
				serviceName: tt.serviceName,
				esNameToEs:  tt.initialES,
			}

			e.OnDelete(tt.deleteES)

			if len(e.esNameToEs) != tt.expectedCount {
				t.Errorf("expected %d endpointslices, got %d", tt.expectedCount, len(e.esNameToEs))
			}

			_, exists := e.esNameToEs[tt.deleteES.Name]
			if exists != tt.shouldExist {
				t.Errorf("expected exists=%v, got exists=%v", tt.shouldExist, exists)
			}
		})
	}
}

func TestVersionMatched(t *testing.T) {
	tests := []struct {
		name       string
		oldVersion string
		newVersion string
		expected   bool
	}{
		{
			name:       "correct version increment",
			oldVersion: "1",
			newVersion: "2",
			expected:   true,
		},
		{
			name:       "version jump",
			oldVersion: "1",
			newVersion: "5",
			expected:   false,
		},
		{
			name:       "version decrease",
			oldVersion: "5",
			newVersion: "3",
			expected:   false,
		},
		{
			name:       "same version",
			oldVersion: "1",
			newVersion: "1",
			expected:   false,
		},
		{
			name:       "large version numbers",
			oldVersion: "99",
			newVersion: "100",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldES := createTestEndpointSlice("es-1", "test-service", tt.oldVersion)
			newES := createTestEndpointSlice("es-1", "test-service", tt.newVersion)

			result := versionMatched(oldES, newES)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for versions %s -> %s",
					tt.expected, result, tt.oldVersion, tt.newVersion)
			}
		})
	}
}

func TestEndpointSlice_DeepCopy(t *testing.T) {
	e := &EndpointSlice{
		serviceName: "test-service",
		esNameToEs:  make(map[string]*discoveryv1.EndpointSlice),
	}

	original := createTestEndpointSlice("es-1", "test-service", "1")
	e.OnAdded(original)

	// Modify the original
	original.Endpoints[0].Addresses[0] = "10.0.0.99"

	// The stored copy should not be affected
	stored := e.esNameToEs["es-1"]
	if stored.Endpoints[0].Addresses[0] == "10.0.0.99" {
		t.Error("DeepCopy not working properly, stored object was modified")
	}
	if stored.Endpoints[0].Addresses[0] != "10.0.0.1" {
		t.Errorf("expected address '10.0.0.1', got '%s'", stored.Endpoints[0].Addresses[0])
	}
}
