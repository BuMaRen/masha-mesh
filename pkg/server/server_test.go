package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BuMaRen/masha-mesh/pkg/controller"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHealthHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctrl := controller.NewServiceController(clientset, "")
	srv := NewServer(8080, ctrl)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	srv.healthHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"healthy"}`
	if rr.Body.String() != expected+"\n" {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestReadyHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctrl := controller.NewServiceController(clientset, "")
	srv := NewServer(8080, ctrl)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	srv.readyHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"ready"}`
	if rr.Body.String() != expected+"\n" {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestRootHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctrl := controller.NewServiceController(clientset, "")
	srv := NewServer(8080, ctrl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	srv.rootHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want application/json", rr.Header().Get("Content-Type"))
	}
}

func TestListServicesHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctrl := controller.NewServiceController(clientset, "")
	srv := NewServer(8080, ctrl)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	rr := httptest.NewRecorder()

	srv.listServicesHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("handler returned wrong content type: got %v want application/json", rr.Header().Get("Content-Type"))
	}
}

func TestGetServiceHandler_InvalidPath(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctrl := controller.NewServiceController(clientset, "")
	srv := NewServer(8080, ctrl)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/invalid", nil)
	rr := httptest.NewRecorder()

	srv.getServiceHandler(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestGetServiceHandler_NotFound(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	ctrl := controller.NewServiceController(clientset, "")
	srv := NewServer(8080, ctrl)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/default/nonexistent", nil)
	rr := httptest.NewRecorder()

	srv.getServiceHandler(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}
}
