package appengine

import (
	"net/http"

	"os"

	"fmt"

	"github.com/googlegenomics/beacon-go/beacon"
)

const (
	apiVersion = "BEACON_API_VERSION"
	project    = "GOOGLE_CLOUD_PROJECT"
	bqTable    = "GOOGLE_BIGQUERY_TABLE"
)

func init() {
	beaconAPI := beacon.BeaconAPI{
		ApiVersion: os.Getenv(apiVersion),
		ProjectID:  os.Getenv(project),
		TableID:    os.Getenv(bqTable),
	}

	if beaconAPI.ProjectID == "" {
		panic(fmt.Sprintf("environment variable %s must be specified", project))
	}
	if beaconAPI.TableID == "" {
		panic(fmt.Sprintf("environment variable %s must be specified", bqTable))
	}

	http.HandleFunc("/", beaconAPI.About)
	http.HandleFunc("/query", beaconAPI.Query)
}
