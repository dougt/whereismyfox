package main

import "encoding/json"
import "net/http"
import "net/http/httptest"
import "strings"
import "testing"
import "github.com/emicklei/go-restful"

type MockPersona struct {
	LoggedIn bool
}

func (self MockPersona) IsLoggedIn(r *http.Request) bool {
	return self.LoggedIn
}

func (self MockPersona) GetLoginName(r *http.Request) string {
	if self.LoggedIn {
		return "ggp@mozilla.com"
	}

	return ""
}

func (self MockPersona) Logout(w http.ResponseWriter, r *http.Request) {
	panic("Logout should not have been called!")
}

func (self MockPersona) Login(verifierURL string, w http.ResponseWriter, r *http.Request) error {
	panic("Login should not have been called!")
}

var gHandlersInitialized = false
func initTestingServer(t *testing.T) func() {
	db, cleanup := initTestDatabase(t)

	gDB = db
	gServerConfig = ServerConfig{}
	gPersona = MockPersona{LoggedIn: true}

	if gHandlersInitialized == false {
		gHandlersInitialized = true
		restful.Add(createDeviceWebService())
		setupPersonaHandlers()
	}

	return func() {
		cleanup()
		gDB = nil
		gServerConfig = ServerConfig{}
		gPersona = nil
	}
}

func doWebServiceRequest(method, url, body string) *httptest.ResponseRecorder {
	return doRequest(method, url, body, restful.DefaultContainer)
}

func doNonWebServiceRequest(method, url, body string) *httptest.ResponseRecorder {
	return doRequest(method, url, body, http.DefaultServeMux)
}

func doRequest(method, url, body string, handler http.Handler) *httptest.ResponseRecorder {
	request, _ := http.NewRequest(method, url, strings.NewReader(body))
	if body != "" {
		request.Header["Content-Type"] = []string{"application/json"}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestUnauthorizedAccess(t *testing.T) {
	cleanup := initTestingServer(t)
	defer cleanup()

	gPersona = MockPersona{LoggedIn: false}

	unauthorized := []string{"/device/", "/device/1"}
	for _, url := range unauthorized {
		response := doWebServiceRequest("GET", url, "")
		if response.Code != http.StatusUnauthorized {
			t.Errorf("Unexpected response code: %d", response.Code)
		}
	}
}

func TestCheckLoggedIn(t *testing.T) {
	cleanup := initTestingServer(t)
	defer cleanup()

	response := doNonWebServiceRequest("GET", "/auth/check", "")
	if response.Code != http.StatusOK {
		t.Errorf("Unexpected response code: %d", response.Code)
	}

	ok := response.Body.String()
	if ok != "ok" {
		t.Errorf("Unexpected response: %d", ok)
	}

	gPersona = MockPersona{LoggedIn: false}
	response = doNonWebServiceRequest("GET", "/auth/check", "")
	if response.Code != http.StatusOK {
		t.Errorf("Unexpected response code: %d", response.Code)
	}

	result := response.Body.String()
	if result != ""  {
		t.Errorf("Unexpected response: %d", result)
	}
}

func TestServeDevicesByUser(t *testing.T) {
	cleanup := initTestingServer(t)
	defer cleanup()

	response := doWebServiceRequest("GET", "/device", "")
	if response.Code != http.StatusOK {
		t.Errorf("Unexpected response code: %d", response.Code)
	}

	expected := []string{"/device/1", "/device/2"}
	result := []string{}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Error("Failed to unmarshal response: " + err.Error())
	}

	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("Got unexpected reply from the server: %#v", result)
		}
	}
}

func TestServeDevice(t *testing.T) {
	cleanup := initTestingServer(t)
	defer cleanup()

	response := doWebServiceRequest("GET", "/device/1", "")
	result := Device{}

	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Error("Failed to unmarshal response: " + err.Error())
	}

	expected := gTestDevices[0]
	if result != expected {
		t.Error("Mismatch in device response: %#v != %#v", result, expected)
	}
}

func TestAddDevice(t *testing.T) {
	cleanup := initTestingServer(t)
	defer cleanup()

	deviceJSON := `{
		"name": "test-device10",
		"endpoint": "http://push.mozilla.com/7eb89e37-df89-4829-a437-7748f9d03910"
	}`

	response := doWebServiceRequest("PUT", "/device/", deviceJSON)
	if response.Code != http.StatusOK {
		t.Errorf("Unexpected response code: %d", response.Code)
	}

	device := Device{}
	if err := json.Unmarshal([]byte(deviceJSON), &device); err != nil {
		t.Errorf("Server accepted invalid device?")
	}

	result := Device{}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Errorf("Failed to unmarshal response: " + err.Error())
	}

	if result.Name != device.Name || result.Endpoint != device.Endpoint {
		t.Errorf("Didn't get the same device back!")
	}
}

func TestUpdateCommands(t *testing.T) {
	cleanup := initTestingServer(t)
	defer cleanup()

	commandsJSON := `[1, 2, 3]`

	response := doWebServiceRequest("PUT", "/device/1/command", commandsJSON)
	if response.Code != http.StatusOK {
		t.Errorf("Unexpected response code: %d", response.Code)
	}
}
