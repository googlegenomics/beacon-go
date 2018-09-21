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

	"github.com/googlegenomics/beacon-go/internal/variants"
	"google.golang.org/appengine"
)

const beaconAPIVersion = "v0.0.1"

var (
	aboutTemplate = template.Must(template.ParseFiles("about.xml"))
)

// Server provides handlers for Beacon API requests.
type Server struct {
	// ProjectID is the GCloud project ID.
	ProjectID string
	// TableID is the ID of the allele BigQuery table to query.
	// Must be provided in the following format: bigquery-project.dataset.table.
	TableID string
}

// Export registers the beacon API endpoint with mux.
func (server *Server) Export(mux *http.ServeMux) {
	mux.Handle("/", forwardOrigin(server.About))
	mux.Handle("/query", forwardOrigin(server.Query))
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

	ctx := appengine.NewContext(r)
	exists, err := query.Execute(ctx, api.ProjectID, api.TableID)
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
			StartMax *int64 ` json:"startMax"`
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
	start, err := getFormValueInt(r, "start")
	if err != nil {
		return fmt.Errorf("parsing start: %v", err)
	}
	params.Start = start

	end, err := getFormValueInt(r, "end")
	if err != nil {
		return fmt.Errorf("parsing end: %v", err)
	}
	params.End = end

	startMin, err := getFormValueInt(r, "startMin")
	if err != nil {
		return fmt.Errorf("parsing startMin: %v", err)
	}
	params.StartMin = startMin

	startMax, err := getFormValueInt(r, "startMax")
	if err != nil {
		return fmt.Errorf("parsing startMax: %v", err)
	}
	params.StartMax = startMax

	endMin, err := getFormValueInt(r, "endMin")
	if err != nil {
		return fmt.Errorf("parsing endMin: %v", err)
	}
	params.EndMin = endMin

	endMax, err := getFormValueInt(r, "endMax")
	if err != nil {
		return fmt.Errorf("parsing endMax: %v", err)
	}
	params.EndMax = endMax
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

type forwardOrigin func(w http.ResponseWriter, req *http.Request)

func (f forwardOrigin) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if origin := req.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	f(w, req)
}
