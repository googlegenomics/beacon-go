package appengine

import (
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/bigquery"
	"github.com/googlegenomics/beacon-go/beacon"
	"google.golang.org/appengine"
)

const (
	project = "GOOGLE_CLOUD_PROJECT"
	bqTable = "GOOGLE_BIGQUERY_TABLE"
	mode    = "BEACON_AUTH_MODE"
)

func init() {
	server := beacon.Server{
		ProjectID:         os.Getenv(project),
		TableID:           os.Getenv(bqTable),
		NewBigQueryClient: newBQClientFunc(),
	}

	if server.ProjectID == "" {
		panic(fmt.Sprintf("environment variable %s must be specified", project))
	}
	if server.TableID == "" {
		panic(fmt.Sprintf("environment variable %s must be specified", bqTable))
	}

	mux := http.NewServeMux()
	server.Export(mux)

	http.HandleFunc("/", mux.ServeHTTP)
}

func newBQClientFunc() beacon.NewBigQueryClientFunc {
	switch os.Getenv(mode) {
	case "auth":
		return newAppEngineClient
	case "", "open":
		return newUnAuthClient
	default:
		panic(fmt.Sprintf("invalid value for %s, specify auth or open", mode))
	}
}

func newAppEngineClient(req *http.Request, projectID string) (*bigquery.Client, error) {
	return beacon.NewClientFromBearerToken(req.WithContext(appengine.NewContext(req)), projectID)
}

func newUnAuthClient(req *http.Request, projectID string) (*bigquery.Client, error) {
	client, err := bigquery.NewClient(appengine.NewContext(req), projectID)
	if err != nil {
		return nil, fmt.Errorf("creating bigquery client: %v", err)
	}
	return client, nil
}
