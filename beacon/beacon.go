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

// Package beacon contains an implementation of GA4GH Beacon API (http://ga4gh.org/#/beacon).
package beacon

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/googlegenomics/beacon-go/internal/query"
	"google.golang.org/appengine"
)

// BeaconAPI implements a GA4GH Beacon API (http://ga4gh.org/#/beacon) backed
// by a Google Cloud BigQuery allele table.
type BeaconAPI struct {
	// ApiVersion the version of the GA4GH Beacon specification the API implements.
	ApiVersion string
	// ProjectID the GCloud project ID.
	ProjectID string
	// TableID the ID of the allele BigQuery table to query.
	// Must be provided in the following format: bigquery-project.dataset.table.
	TableID string
}

var (
	aboutTemplate = template.Must(template.ParseFiles("about.xml"))
)

func (api *BeaconAPI) About(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, fmt.Sprintf("HTTP method %s not supported", r.Method), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	aboutTemplate.Execute(w, api)
}

func (api *BeaconAPI) Query(w http.ResponseWriter, r *http.Request) {
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

func parseInput(r *http.Request) (*query.Query, error) {
	var query query.Query
	query.RefName = r.FormValue("chromosome")
	query.Allele = r.FormValue("allele")

	coord, err := getFormValueInt(r, "coordinate")
	if err != nil {
		return nil, fmt.Errorf("parsing coordinate: %v", err)
	}
	query.Coord = coord

	return &query, nil
}

func getFormValueInt(r *http.Request, key string) (*int64, error) {
	str := r.FormValue(key)
	if str == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing int value: %v", err)
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
