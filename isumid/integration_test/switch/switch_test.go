package test_e2e_switch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kajikentaro/isucon-middleware/isumid"
	utils "github.com/kajikentaro/isucon-middleware/isumid/integration_test"
	"github.com/kajikentaro/isucon-middleware/isumid/models"
	"github.com/kajikentaro/isucon-middleware/isumid/settings"
	"github.com/stretchr/testify/assert"
)

var OUTPUT_DIR = filepath.Join(os.TempDir(), uuid.NewString())
var PORT_NUMBER = 8082
var HTTP_ADDRESS = fmt.Sprintf(":%d", PORT_NUMBER)
var URL_LIST = utils.GetUrlList(PORT_NUMBER)

func TestMain(m *testing.M) {
	fmt.Println("test dir:", OUTPUT_DIR)
	m.Run()
}

func TestRecordOnStart(t *testing.T) {
	// start server
	rec := isumid.New(&settings.Setting{
		OutputDir:     OUTPUT_DIR,
		RecordOnStart: true,
	})
	mux := http.NewServeMux()
	mux.Handle("/", rec.Middleware(http.HandlerFunc(utils.SampleHandler)))
	srv := &http.Server{Addr: HTTP_ADDRESS, Handler: mux}
	go utils.StartServer(srv)
	time.Sleep(time.Second)
	defer utils.StopServer(srv)

	previousExpected := len(utils.FetchAllTransactions(t, PORT_NUMBER))

	sendSampleRequest(t)

	// fetch result
	actual := len(utils.FetchAllTransactions(t, PORT_NUMBER))
	expected := previousExpected + 1
	assert.Equal(t, expected, actual)
}

func TestDoNotRecordOnStart(t *testing.T) {
	// start server
	rec := isumid.New(&settings.Setting{
		OutputDir:     OUTPUT_DIR,
		RecordOnStart: false,
	})
	mux := http.NewServeMux()
	mux.Handle("/", rec.Middleware(http.HandlerFunc(utils.SampleHandler)))
	srv := &http.Server{Addr: HTTP_ADDRESS, Handler: mux}
	go utils.StartServer(srv)
	time.Sleep(time.Second)
	defer utils.StopServer(srv)

	previousExpected := len(utils.FetchAllTransactions(t, PORT_NUMBER))

	sendSampleRequest(t)

	// fetch result
	actual := len(utils.FetchAllTransactions(t, PORT_NUMBER))
	expected := previousExpected
	assert.Equal(t, expected, actual)
}

func fetchIsRecording(t *testing.T) bool {
	res, err := http.Get(URL_LIST.IsRecording)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode, "status code should be 200")

	isRecording := models.IsRecordingResponse{}
	err = json.NewDecoder(res.Body).Decode(&isRecording)
	assert.NoError(t, err)

	return isRecording.IsRecording
}

func TestStartAndStopRecording(t *testing.T) {
	// start server
	rec := isumid.New(&settings.Setting{
		OutputDir:     OUTPUT_DIR,
		RecordOnStart: false,
	})
	mux := http.NewServeMux()
	mux.Handle("/", rec.Middleware(http.HandlerFunc(utils.SampleHandler)))
	srv := &http.Server{Addr: HTTP_ADDRESS, Handler: mux}
	go utils.StartServer(srv)
	time.Sleep(time.Second)
	defer utils.StopServer(srv)

	{
		previousExpected := len(utils.FetchAllTransactions(t, 8082))

		// turn on recording
		res, err := http.Post(URL_LIST.StartRecording, "text/plain", nil)
		assert.NoError(t, err)
		defer res.Body.Close()
		assert.Equal(t, 200, res.StatusCode, "status code should be 200")

		sendSampleRequest(t)

		// fetch result
		actual := len(utils.FetchAllTransactions(t, 8082))
		expected := previousExpected + 1
		assert.Equal(t, expected, actual)
		assert.Equal(t, true, fetchIsRecording(t))
	}

	{
		previousExpected := len(utils.FetchAllTransactions(t, PORT_NUMBER))

		// turn off recording
		res, err := http.Post(URL_LIST.StopRecording, "text/plain", nil)
		assert.NoError(t, err)
		defer res.Body.Close()
		assert.Equal(t, 200, res.StatusCode, "status code should be 200")

		sendSampleRequest(t)

		// fetch result
		actual := len(utils.FetchAllTransactions(t, PORT_NUMBER))
		expected := previousExpected
		assert.Equal(t, expected, actual)
		assert.Equal(t, false, fetchIsRecording(t))
	}
}

func TestAutoStart(t *testing.T) {
	// start server
	rec := isumid.New(&settings.Setting{
		OutputDir:     OUTPUT_DIR,
		RecordOnStart: false,
		AutoStart: &settings.AutoSwitch{
			TriggerEndpoint: "/trigger",
			AfterSec:        1,
		},
	})
	mux := http.NewServeMux()
	mux.Handle("/", rec.Middleware(http.HandlerFunc(utils.SampleHandler)))
	srv := &http.Server{Addr: HTTP_ADDRESS, Handler: mux}
	go utils.StartServer(srv)
	time.Sleep(time.Second)
	defer utils.StopServer(srv)

	{
		previousExpected := len(utils.FetchAllTransactions(t, PORT_NUMBER))

		// send superfluous request
		requestBody := "Hello World"
		url := URL_LIST.UrlOrigin + "/trigger/foo"
		res, err := http.Post(url, "text/plain", bytes.NewBufferString(requestBody))
		assert.NoError(t, err)
		defer res.Body.Close()
		assert.Equal(t, 200, res.StatusCode, "status code should be 200")

		// fetch result
		actual := len(utils.FetchAllTransactions(t, PORT_NUMBER))
		expected := previousExpected
		assert.Equal(t, expected, actual)
	}

	{
		expected := len(utils.FetchAllTransactions(t, PORT_NUMBER))

		// send request that is trigger to turn off recording after 1 sec
		sendSampleRequestTrigger(t)

		sendSampleRequest(t)

		time.Sleep(1 * time.Second)

		sendSampleRequest(t)
		expected++

		// fetch result
		actual := len(utils.FetchAllTransactions(t, PORT_NUMBER))
		assert.Equal(t, expected, actual)
	}
}

func TestAutoStop(t *testing.T) {
	// start server
	rec := isumid.New(&settings.Setting{
		OutputDir:     OUTPUT_DIR,
		RecordOnStart: true,
		AutoStop: &settings.AutoSwitch{
			TriggerEndpoint: "/trigger",
			AfterSec:        1,
		},
	})
	mux := http.NewServeMux()
	mux.Handle("/", rec.Middleware(http.HandlerFunc(utils.SampleHandler)))
	srv := &http.Server{Addr: HTTP_ADDRESS, Handler: mux}
	go utils.StartServer(srv)
	time.Sleep(time.Second)
	defer utils.StopServer(srv)

	{
		previousExpected := len(utils.FetchAllTransactions(t, PORT_NUMBER))

		// send superfluous request
		requestBody := "Hello World"
		url := URL_LIST.UrlOrigin + "/trigger/foo"
		res, err := http.Post(url, "text/plain", bytes.NewBufferString(requestBody))
		assert.NoError(t, err)
		defer res.Body.Close()
		assert.Equal(t, 200, res.StatusCode, "status code should be 200")

		// fetch result
		actual := len(utils.FetchAllTransactions(t, PORT_NUMBER))
		expected := previousExpected + 1
		assert.Equal(t, expected, actual)
	}

	{
		expected := len(utils.FetchAllTransactions(t, PORT_NUMBER))

		// send request that is trigger to turn off recording after 1 sec
		sendSampleRequestTrigger(t)
		expected++

		sendSampleRequest(t)
		expected++

		time.Sleep(1 * time.Second)

		sendSampleRequest(t)

		// fetch result
		actual := len(utils.FetchAllTransactions(t, PORT_NUMBER))
		assert.Equal(t, expected, actual)
	}
}

func sendSampleRequest(t *testing.T) {
	requestBody := "Hello World"
	url := URL_LIST.UrlOrigin + "/"
	res, err := http.Post(url, "text/plain", bytes.NewBufferString(requestBody))
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, 200, res.StatusCode, "status code should be 200")
}

func sendSampleRequestTrigger(t *testing.T) {
	requestBody := "Hello World"
	url := URL_LIST.UrlOrigin + "/trigger"
	res, err := http.Post(url, "text/plain", bytes.NewBufferString(requestBody))
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, 200, res.StatusCode, "status code should be 200")
}
