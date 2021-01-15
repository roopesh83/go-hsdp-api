package cdr_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/fhir/go/jsonformat"

	"github.com/philips-software/go-hsdp-api/cdr"

	"github.com/philips-software/go-hsdp-api/iam"
	"github.com/stretchr/testify/assert"
)

var (
	muxIAM    *http.ServeMux
	serverIAM *httptest.Server
	muxIDM    *http.ServeMux
	serverIDM *httptest.Server
	muxCDR    *http.ServeMux
	serverCDR *httptest.Server

	iamClient    *iam.Client
	cdrClient    *cdr.Client
	cdrOrgID     = "48a0183d-a588-41c2-9979-737d15e9e860"
	userUUID     = "e7fecbb2-af8c-47c9-a662-5b046e048bc5"
	timeZone     = "Europe/Amsterdam"
	token        string
	refreshToken string
	ma           *jsonformat.Marshaller
	um           *jsonformat.Unmarshaller
)

func setup(t *testing.T) func() {
	muxIAM = http.NewServeMux()
	serverIAM = httptest.NewServer(muxIAM)
	muxIDM = http.NewServeMux()
	serverIDM = httptest.NewServer(muxIDM)
	muxCDR = http.NewServeMux()
	serverCDR = httptest.NewServer(muxCDR)

	var err error
	token = "44d20214-7879-4e35-923d-f9d4e01c9746"
	refreshToken = "31f1a449-ef8e-4bfc-a227-4f2353fde547"

	iamClient, err = iam.NewClient(nil, &iam.Config{
		OAuth2ClientID: "TestClient",
		OAuth2Secret:   "Secret",
		SharedKey:      "SharedKey",
		SecretKey:      "SecretKey",
		IAMURL:         serverIAM.URL,
		IDMURL:         serverIDM.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create iamCleitn: %v", err)
	}
	token := "44d20214-7879-4e35-923d-f9d4e01c9746"

	muxIAM.HandleFunc("/authorize/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected ‘POST’ request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
    "scope": "mail tdr.contract tdr.dataitem",
    "access_token": "`+token+`",
    "refresh_token": "31f1a449-ef8e-4bfc-a227-4f2353fde547",
    "expires_in": 1799,
    "token_type": "Bearer"
}`)
	})
	muxIAM.HandleFunc("/authorize/oauth2/introspect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected ‘POST’ request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{
  "active": true,
  "scope": "auth_iam_organization auth_iam_introspect mail openid profile cn",
  "username": "ronswanson",
  "exp": 1592073485,
  "sub": "`+userUUID+`",
  "iss": "https://iam-client-test.us-east.philips-healthsuite.com/oauth2/access_token",
  "organizations": {
    "managingOrganization": "`+cdrOrgID+`",
    "organizationList": [
      {
        "organizationId": "`+cdrOrgID+`",
        "permissions": [
          "USER.READ",
          "GROUP.WRITE",
          "DEVICE.READ",
          "CLIENT.SCOPES",
          "AMS_ACCESS.ALL",
          "PKI_CRL_CONFIGURATION.READ",
          "PKI_CERT.ISSUE",
          "PKI_CERT.READ",
          "PKI_CERTS.LIST",
          "PKI_CERTROLE.LIST",
          "PKI_CERTROLE.READ",
          "PKI_URLS.READ",
          "PKI_CRL.ROTATE",
          "PKI_CRL.CONFIGURE",
          "PKI_CERT.SIGN",
          "PKI_CERT.REVOKE",
          "PKI_URLS.CONFIGURE"
        ],
        "organizationName": "PawneeOrg",
        "groups": [
          "AdminGroup"
        ],
        "roles": [
          "ADMIN",
          "PKIROLE"
        ]
      }
    ]
  },
  "client_id": "testclientid",
  "token_type": "Bearer",
  "identity_type": "user"
}`)
	})

	// Login immediately so we can create cdrClient
	err = iamClient.Login("username", "password")
	assert.Nil(t, err)

	cdrClient, err = cdr.NewClient(iamClient, &cdr.Config{
		CDRURL:    serverCDR.URL,
		RootOrgID: cdrOrgID,
		TimeZone:  timeZone,
	})
	if !assert.Nil(t, err) {
		t.Fatalf("invalid client")
	}
	ma, err = jsonformat.NewMarshaller(false, "", "", jsonformat.STU3)
	if !assert.Nil(t, err) {
		t.Fatalf("failed to create marshaller")
	}
	um, err = jsonformat.NewUnmarshaller("Europe/Amsterdam", jsonformat.STU3)
	if !assert.Nil(t, err) {
		t.Fatalf("failed to create unmarshaller")
	}

	return func() {
		serverIAM.Close()
		serverIDM.Close()
		serverCDR.Close()
	}
}

func TestLogin(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	token := "44d20214-7879-4e35-923d-f9d4e01c9746"

	err := iamClient.Login("username", "password")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, token, iamClient.Token())
}

func TestDebug(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	cdrClient, err = cdr.NewClient(iamClient, &cdr.Config{
		CDRURL:   serverCDR.URL,
		DebugLog: tmpfile.Name(),
	})
	if !assert.Nil(t, err) {
		return
	}

	defer cdrClient.Close()
	defer os.Remove(tmpfile.Name()) // clean up

	err = iamClient.Login("username", "password")
	if !assert.Nil(t, err) {
		return
	}

	fi, err := tmpfile.Stat()
	assert.Nil(t, err)
	assert.NotEqual(t, 0, fi.Size(), "Expected something to be written to DebugLog")
}

func TestEndpoints(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	rootOrgID := "foo"

	cdrClient, err := cdr.NewClient(iamClient, &cdr.Config{
		CDRURL:    serverCDR.URL,
		RootOrgID: rootOrgID,
	})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, cdrClient) {
		return
	}
	endpoint := cdrClient.GetEndpointURL()
	assert.Equal(t, serverCDR.URL+"/store/fhir/", cdrClient.GetFHIRStoreURL())
	assert.Equal(t, serverCDR.URL+"/store/fhir/"+rootOrgID, endpoint)

	assert.Nil(t, cdrClient.SetEndpointURL(endpoint))
	assert.Equal(t, serverCDR.URL+"/store/fhir/"+rootOrgID, cdrClient.GetEndpointURL())

}