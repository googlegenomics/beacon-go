/*
 * Copyright (C) 2015 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 */

// Package beacon implements a GA4GH Beacon API (https://github.com/ga4gh-beacon/specification/blob/master/beacon.md).
package beacon

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"cloud.google.com/go/bigquery"
	"github.com/googlegenomics/beacon-go/internal/variants"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/appengine"
)

const beaconAPIVersion = "v0.0.1"

var (
	aboutTemplate = template.Must(template.ParseFiles("about.xml"))
)

// AuthenticationMode defines what authentication credentials the server uses to connect to
// BigQuery.
type AuthenticationMode uint

const (
	// ServiceAuth will configure the server to use its service account credentials to access the
	// BigQuery datasets.
	ServiceAuth AuthenticationMode = iota
	// UserAuth will configure the server to use the authentication header provided in the request to
	// access the BigQuery datasets.
	UserAuth
)

// Server provides handlers for Beacon API requests.
type Server struct {
	// ProjectID is the GCloud project ID.
	ProjectID string
	// TableID is the ID of the allele BigQuery table to query.
	// Must be provided in the following format: bigquery-project.dataset.table.
	TableID string
	// AuthMode determines the authentication provider for the BigQuery client.
	AuthMode AuthenticationMode
}

// Export registers the beacon API endpoint with mux.
func (server *Server) Export(mux *http.ServeMux) {
	mux.Handle("/", &forwardOrigin{server.About, []string{"GET"}})
	mux.Handle("/query", &forwardOrigin{server.Query, []string{"GET", "POST"}})
}

// About retrieves all the necessary information on the beacon and the API.
func (api *Server) About(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, fmt.Sprintf("HTTP method %s not supported", r.Method), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	aboutTemplate.Execute(w, map[string]string{
		"APIVersion": beaconAPIVersion,
		"TableID":    api.TableID,
	})
}

// Query retrieves whether the requested allele exists in the dataset.
func (api *Server) Query(w http.ResponseWriter, r *http.Request) {
	query, err := parseInput(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("parsing input: %v", err), http.StatusBadRequest)
		return
	}

	if err := query.ValidateInput(); err != nil {
		http.Error(w, fmt.Sprintf("validating input: %v", err), http.StatusBadRequest)
		return
	}

	client, err := api.newBQClient(r, api.ProjectID)
	if err != nil {
		http.Error(w, fmt.Sprintf("creating bigquery client: %v", err), http.StatusBadRequest)
		return
	}

	exists, err := query.Execute(r.Context(), client, api.TableID)
	if err != nil {
		http.Error(w, fmt.Sprintf("computing result: %v", err), http.StatusInternalServerError)
		return
	}
	writeResponse(w, exists)
}

func parseInput(r *http.Request) (*variants.Query, error) {
	switch r.Method {
	case "GET":
		var query variants.Query
		query.RefName = r.FormValue("chromosome")
		query.Allele = r.FormValue("allele")
		if err := parseFormCoordinates(r, &query); err != nil {
			return nil, fmt.Errorf("parsing referenceBases: %v", err)
		}
		return &query, nil
	case "POST":
		var params struct {
			RefName  string `json:"chromosome"`
			Allele   string `json:"allele"`
			Start    *int64 `json:"start"`
			End      *int64 `json:"end"`
			StartMin *int64 `json:"startMin"`
			StartMax *int64 `json:"startMax"`
			EndMin   *int64 `json:"endMin"`
			EndMax   *int64 `json:"endMax"`
		}
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &params); err != nil {
			return nil, fmt.Errorf("decoding request body: %v", err)
		}

		return &variants.Query{
			RefName:  params.RefName,
			Allele:   params.Allele,
			Start:    params.Start,
			End:      params.End,
			StartMin: params.StartMin,
			StartMax: params.StartMax,
			EndMin:   params.EndMin,
			EndMax:   params.EndMax,
		}, nil
	default:
		return nil, errors.New(fmt.Sprintf("HTTP method %s not supported", r.Method))
	}
}

func parseFormCoordinates(r *http.Request, params *variants.Query) error {
	fields := map[string]**int64{
		"start":    &params.Start,
		"end":      &params.End,
		"startMin": &params.StartMin,
		"startMax": &params.StartMax,
		"endMin":   &params.EndMin,
		"endMax":   &params.EndMax,
	}

	for name, field := range fields {
		v, err := getFormValueInt(r, name)
		if err != nil {
			return fmt.Errorf("parsing %s: %v", name, err)
		}
		*field = v
	}
	return nil
}

func getFormValueInt(r *http.Request, key string) (*int64, error) {
	str := r.FormValue(key)
	if str == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing value as integer: %v", err)
	}
	return &value, nil
}

func writeResponse(w http.ResponseWriter, exists bool) {
	type beaconResponse struct {
		XMLName struct{} `xml:"BEACONResponse"`
		Exists  bool     `xml:"exists"`
	}
	var resp beaconResponse
	resp.Exists = exists

	w.Header().Set("Content-Type", "application/xml")
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(resp)
}

type forwardOrigin struct {
	handler func(w http.ResponseWriter, req *http.Request)
	methods []string
}

func (f *forwardOrigin) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if origin := req.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if req.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(f.methods, ","))
			w.Header().Set("Access-Control-Allow-Headers", "Authorization")
			return
		}
	}
	f.handler(w, req)
}

func (api *Server) newBQClient(req *http.Request, projectID string) (*bigquery.Client, error) {
	switch api.AuthMode {
	case ServiceAuth:
		return bigquery.NewClient(appengine.NewContext(req), projectID)
	case UserAuth:
		return newClientFromBearerToken(req.WithContext(appengine.NewContext(req)), projectID)
	default:
		return nil, fmt.Errorf("invalid value %d for server authentication mode", api.AuthMode)
	}
}

func newClientFromBearerToken(req *http.Request, projectID string) (*bigquery.Client, error) {
	authorization := req.Header.Get("Authorization")

	fields := strings.Split(authorization, " ")
	if len(fields) != 2 || fields[0] != "Bearer" {
		return nil, errors.New("missing or invalid authentication token")
	}

	token := oauth2.Token{
		TokenType:   fields[0],
		AccessToken: fields[1],
	}

	client, err := bigquery.NewClient(req.Context(), projectID, option.WithTokenSource(oauth2.StaticTokenSource(&token)))
	if err != nil {
		return nil, fmt.Errorf("creating bigquery client: %v", err)
	}

	return client, nil
}
