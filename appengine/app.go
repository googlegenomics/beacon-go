package appengine

import (
	"fmt"
	"net/http"
	"os"

	"github.com/googlegenomics/beacon-go/beacon"
)

const (
	project = "GOOGLE_CLOUD_PROJECT"
	bqTable = "GOOGLE_BIGQUERY_TABLE"
)

func init() {
	server := beacon.Server{
		ProjectID: os.Getenv(project),
		TableID:   os.Getenv(bqTable),
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
