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

// Package beacon implements a GA4GH Beacon (http://ga4gh.org/#/beacon) backed
// by the Google Genomics Variants service search API.
package beacon

import (
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"

	"encoding/json"
	"io/ioutil"

	"google.golang.org/appengine"
)

type beaconConfig struct {
	ApiVersion string
	ProjectID  string
	TableID    string
}

const (
	apiVersionKey = "BEACON_API_VERSION"
	projectKey    = "GOOGLE_CLOUD_PROJECT"
	bqTableKey    = "GOOGLE_BIGQUERY_TABLE"
)

var (
	aboutTemplate = template.Must(template.ParseFiles("about.xml"))
	config        = beaconConfig{
		ApiVersion: os.Getenv(apiVersionKey),
		ProjectID:  os.Getenv(projectKey),
		TableID:    os.Getenv(bqTableKey),
	}
)

func init() {
	http.HandleFunc("/", aboutBeacon)
	http.HandleFunc("/query", query)
}

func aboutBeacon(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, fmt.Sprintf("HTTP method %s not supported", r.Method), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	aboutTemplate.Execute(w, config)
}

func query(w http.ResponseWriter, r *http.Request) {
	if err := validateServerConfig(); err != nil {
		http.Error(w, fmt.Sprintf("validating server configuration: %v", err), http.StatusInternalServerError)
		return
	}

	query, err := parseInput(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("parsing input: %v", err), http.StatusBadRequest)
		return
	}

	if err := query.validateInput(); err != nil {
		http.Error(w, fmt.Sprintf("validating input: %v", err), http.StatusBadRequest)
		return
	}

	ctx := appengine.NewContext(r)
	exists, err := query.Execute(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("computing result: %v", err), http.StatusInternalServerError)
		return
	}

	if err := writeResponse(w, exists); err != nil {
		http.Error(w, fmt.Sprintf("writing response: %v", err), http.StatusInternalServerError)
		return
	}
}

func validateServerConfig() error {
	if config.ProjectID == "" {
		return fmt.Errorf("%s must be specified", projectKey)
	}
	if config.TableID == "" {
		return fmt.Errorf("%s must be specified", bqTableKey)
	}
	return nil
}

func parseInput(r *http.Request) (*Query, error) {
	if r.Method == "GET" {
		var query Query
		query.RefName = r.FormValue("referenceName")
		query.RefBases = r.FormValue("referenceBases")
		if err := parseFormCoordinates(r, &query); err != nil {
			return nil, fmt.Errorf("parsing referenceBases: %v", err)
		}
		return &query, nil
	} else if r.Method == "POST" {
		var params struct {
			RefName  string `json:"referenceName"`
			RefBases string `json:"referenceBases"`
			Start    *int64 `json:"start"`
			End      *int64 `json:"end"`
		}
		body, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(body, &params); err != nil {
			return nil, fmt.Errorf("decoding request body: %v", err)
		}
		return &Query{
			RefName:  params.RefName,
			RefBases: params.RefBases,
			Start:    params.Start,
			End:      params.End,
		}, nil
	}
	return nil, errors.New(fmt.Sprintf("HTTP method %s not supported", r.Method))
}

func parseFormCoordinates(r *http.Request, params *Query) error {
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
	return nil
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

func writeResponse(w http.ResponseWriter, exists bool) error {
	type beaconResponse struct {
		XMLName struct{} `xml:"BEACONResponse"`
		Exists  bool     `xml:"exists"`
	}
	var resp beaconResponse
	resp.Exists = exists

	w.Header().Set("Content-Type", "application/xml")
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(resp); err != nil {
		return fmt.Errorf("serializing response: %v", err)
	}
	return nil
}
